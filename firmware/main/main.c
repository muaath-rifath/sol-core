#include <stdio.h>
#include <string.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_event.h"
#include "esp_log.h"
#include "esp_system.h"
#include "driver/gpio.h"
#include "mqtt_client.h"
#include "nvs_flash.h"
#include "cJSON.h"

#include "wifi.h"
#include "led_control.h"
#include "ota.h"
#include "runtime_config.h"
#include "dht_sensor.h"
#include "smart_plug.h"

static const char *TAG = "main";

static esp_mqtt_client_handle_t s_mqtt_client = NULL;

static char s_topic_state[128];
static char s_topic_cmd[128];
static char s_topic_ack[128];
static char s_lwt_payload[160];
static char s_device_id[RUNTIME_DEVICE_ID_MAX_LEN + 1];
static TickType_t s_last_state_publish_tick = 0;
static bool s_mqtt_connected = false;
static runtime_template_mode_t s_template_mode = RUNTIME_TEMPLATE_RGB_LED;
static int s_relay_pins[RUNTIME_RELAY_CHANNELS_MAX] = {-1, -1, -1, -1};
static bool s_relay_logical_states[RUNTIME_RELAY_CHANNELS_MAX] = {false, false, false, false};

#define STATE_HEARTBEAT_INTERVAL_MS 2000

static bool template_supports_relay(void);

static void publish_state(bool online)
{
    if (!s_mqtt_client) {
        return;
    }

    cJSON *state = cJSON_CreateObject();
    if (!state) {
        return;
    }

    cJSON_AddStringToObject(state, "deviceId", s_device_id);
    cJSON_AddStringToObject(state, "name", "ESP32 LED Controller");
    cJSON_AddBoolToObject(state, "online", online);
    cJSON_AddStringToObject(state, "templateId", runtime_get_template_id());
    cJSON_AddNumberToObject(state, "templateMode", (double)s_template_mode);
    cJSON_AddNumberToObject(state, "ts", (double)esp_log_timestamp());
    cJSON_AddStringToObject(state, "ip_address", wifi_get_ip());

    if (template_supports_relay()) {
        cJSON *relays = cJSON_CreateArray();
        for (int i = 0; i < RUNTIME_RELAY_CHANNELS_MAX; i++) {
            if (s_relay_pins[i] != -1) {
                cJSON_AddItemToArray(relays, cJSON_CreateBool(s_relay_logical_states[i]));
            } else {
                cJSON_AddItemToArray(relays, cJSON_CreateNull());
            }
        }
        cJSON_AddItemToObject(state, "relays", relays);
    }

    char *payload = cJSON_PrintUnformatted(state);
    cJSON_Delete(state);
    if (!payload) {
        return;
    }

    int msg_id = esp_mqtt_client_publish(s_mqtt_client, s_topic_state, payload, 0, 1, 1);
    if (msg_id < 0) {
        ESP_LOGW(TAG, "Failed to publish %s state", online ? "online" : "offline");
    }
    cJSON_free(payload);
}

static bool template_supports_relay(void)
{
    return s_template_mode == RUNTIME_TEMPLATE_RELAY_SINGLE ||
           s_template_mode == RUNTIME_TEMPLATE_RELAY_4CH_GPIO ||
           s_template_mode == RUNTIME_TEMPLATE_SMART_PLUG;
}

static bool relay_channel_valid(uint8_t channel)
{
    return channel < RUNTIME_RELAY_CHANNELS_MAX && s_relay_pins[channel] >= 0;
}

static esp_err_t relay_write_channel(uint8_t channel, bool logical_on)
{
    if (!relay_channel_valid(channel)) {
        return ESP_ERR_INVALID_ARG;
    }

    bool active_low = runtime_is_relay_active_low(channel);
    int level = logical_on ? (active_low ? 0 : 1) : (active_low ? 1 : 0);
    
    if (s_template_mode == RUNTIME_TEMPLATE_SMART_PLUG && channel == 0) {
        smart_plug_set_state(logical_on);
    }
    
    s_relay_logical_states[channel] = logical_on;
    return gpio_set_level((gpio_num_t)s_relay_pins[channel], level);
}

static void init_template_runtime(void)
{
    s_template_mode = runtime_get_template_mode();

    for (uint8_t i = 0; i < RUNTIME_RELAY_CHANNELS_MAX; i++) {
        s_relay_pins[i] = runtime_get_relay_pin(i);
        if (s_relay_pins[i] < 0) {
            continue;
        }
        gpio_reset_pin((gpio_num_t)s_relay_pins[i]);
        gpio_set_direction((gpio_num_t)s_relay_pins[i], GPIO_MODE_OUTPUT);
        s_relay_logical_states[i] = false;
        gpio_set_level((gpio_num_t)s_relay_pins[i], runtime_is_relay_active_low(i) ? 1 : 0);
        ESP_LOGI(TAG, "Relay channel %u mapped to GPIO %d (activeLow=%s)",
                 (unsigned)(i + 1), s_relay_pins[i], runtime_is_relay_active_low(i) ? "true" : "false");
    }

    ESP_LOGI(TAG, "Template selected: %s (mode=%u)", runtime_get_template_id(), (unsigned)s_template_mode);
}

static void publish_ack(const char *request_id, bool ok, const char *message)
{
    if (!s_mqtt_client || !request_id) {
        return;
    }

    cJSON *root = cJSON_CreateObject();
    if (!root) {
        return;
    }

    cJSON_AddStringToObject(root, "requestId", request_id);
    cJSON_AddBoolToObject(root, "ok", ok);
    cJSON_AddStringToObject(root, "message", message ? message : (ok ? "OK" : "Error"));
    cJSON_AddNumberToObject(root, "ts", (double)esp_log_timestamp());

    char *payload = cJSON_PrintUnformatted(root);
    cJSON_Delete(root);
    if (!payload) {
        return;
    }

    int msg_id = esp_mqtt_client_publish(s_mqtt_client, s_topic_ack, payload, 0, 1, 0);
    if (msg_id < 0) {
        ESP_LOGW(TAG, "Failed to publish ack");
    }
    cJSON_free(payload);
}

static void ota_task(void *param)
{
    char *url = (char *)param;
    if (!url) {
        vTaskDelete(NULL);
        return;
    }

    esp_err_t ret = ota_start(url);
    if (ret != ESP_OK) {
        ESP_LOGE(TAG, "OTA failed: %s", esp_err_to_name(ret));
    }
    free(url);
    vTaskDelete(NULL);
}

static void handle_command_json(const char *json_text)
{
    if (!json_text) {
        return;
    }

    cJSON *root = cJSON_Parse(json_text);
    if (!root) {
        ESP_LOGW(TAG, "Invalid JSON command payload");
        return;
    }

    const cJSON *request_id = cJSON_GetObjectItemCaseSensitive(root, "requestId");
    const cJSON *cmd = cJSON_GetObjectItemCaseSensitive(root, "cmd");
    const cJSON *args = cJSON_GetObjectItemCaseSensitive(root, "args");

    const char *req_id = cJSON_IsString(request_id) ? request_id->valuestring : NULL;
    if (!req_id || !cJSON_IsString(cmd)) {
        cJSON_Delete(root);
        return;
    }

    if (strcmp(cmd->valuestring, "set_led_color") == 0) {
        int r = 0;
        int g = 0;
        int b = 0;
        if (cJSON_IsObject(args)) {
            const cJSON *jr = cJSON_GetObjectItemCaseSensitive(args, "r");
            const cJSON *jg = cJSON_GetObjectItemCaseSensitive(args, "g");
            const cJSON *jb = cJSON_GetObjectItemCaseSensitive(args, "b");
            if (cJSON_IsNumber(jr)) r = jr->valueint;
            if (cJSON_IsNumber(jg)) g = jg->valueint;
            if (cJSON_IsNumber(jb)) b = jb->valueint;
        }

        if (r < 0) r = 0;
        if (r > 255) r = 255;
        if (g < 0) g = 0;
        if (g > 255) g = 255;
        if (b < 0) b = 0;
        if (b > 255) b = 255;

        led_set_color((uint8_t)r, (uint8_t)g, (uint8_t)b);
        publish_ack(req_id, true, "LED color set successfully");
    } else if (strcmp(cmd->valuestring, "turn_led_off") == 0) {
        led_off();
        publish_ack(req_id, true, "LED turned off");
    } else if (strcmp(cmd->valuestring, "relay_set") == 0) {
        if (!template_supports_relay()) {
            publish_ack(req_id, false, "relay_set is not enabled for current template");
        } else {
            const cJSON *channel = cJSON_IsObject(args) ? cJSON_GetObjectItemCaseSensitive(args, "channel") : NULL;
            const cJSON *on = cJSON_IsObject(args) ? cJSON_GetObjectItemCaseSensitive(args, "on") : NULL;
            if (!cJSON_IsNumber(channel) || !cJSON_IsBool(on)) {
                publish_ack(req_id, false, "relay_set requires numeric channel and boolean on");
            } else {
                int requested = channel->valueint;
                if (requested < 1 || requested > RUNTIME_RELAY_CHANNELS_MAX) {
                    publish_ack(req_id, false, "relay channel must be between 1 and 4");
                } else {
                    uint8_t idx = (uint8_t)(requested - 1);
                    if (!relay_channel_valid(idx)) {
                        publish_ack(req_id, false, "relay channel is not configured");
                    } else if (relay_write_channel(idx, cJSON_IsTrue(on)) != ESP_OK) {
                        publish_ack(req_id, false, "failed to write relay GPIO");
                    } else {
                        publish_ack(req_id, true, "relay state updated");
                    }
                }
            }
        }
    } else if (strcmp(cmd->valuestring, "ota_update") == 0) {
        const cJSON *url = cJSON_IsObject(args) ? cJSON_GetObjectItemCaseSensitive(args, "url") : NULL;
        if (!cJSON_IsString(url) || !url->valuestring || strlen(url->valuestring) == 0) {
            publish_ack(req_id, false, "url argument required");
        } else {
            char *url_copy = strdup(url->valuestring);
            if (!url_copy) {
                publish_ack(req_id, false, "failed to allocate ota url");
            } else {
                BaseType_t ok = xTaskCreate(ota_task, "ota", 8192, url_copy, 5, NULL);
                if (ok != pdPASS) {
                    free(url_copy);
                    publish_ack(req_id, false, "failed to start ota task");
                } else {
                    publish_ack(req_id, true, "OTA update started, device will reboot shortly");
                }
            }
        }
    } else {
        publish_ack(req_id, false, "unknown command");
    }

    cJSON_Delete(root);
}

static void mqtt_event_handler(void *handler_args, esp_event_base_t base, int32_t event_id, void *event_data)
{
    (void)handler_args;
    (void)base;

    esp_mqtt_event_handle_t event = event_data;
    switch ((esp_mqtt_event_id_t)event_id) {
        case MQTT_EVENT_CONNECTED: {
            ESP_LOGI(TAG, "MQTT connected");
            s_mqtt_connected = true;
            int sub_id = esp_mqtt_client_subscribe(s_mqtt_client, s_topic_cmd, 1);
            if (sub_id < 0) {
                ESP_LOGE(TAG, "Subscribe failed for %s", s_topic_cmd);
            }

            publish_state(true);
            s_last_state_publish_tick = xTaskGetTickCount();
            break;
        }
        case MQTT_EVENT_DISCONNECTED:
            ESP_LOGW(TAG, "MQTT disconnected");
            s_mqtt_connected = false;
            break;
        case MQTT_EVENT_DATA: {
            if (!event->topic || event->topic_len <= 0 || !event->data || event->data_len <= 0) {
                break;
            }

            if ((int)strlen(s_topic_cmd) != event->topic_len ||
                strncmp(event->topic, s_topic_cmd, event->topic_len) != 0) {
                break;
            }

            char *payload = calloc(1, event->data_len + 1);
            if (!payload) {
                break;
            }
            memcpy(payload, event->data, event->data_len);
            handle_command_json(payload);
            free(payload);
            break;
        }
        case MQTT_EVENT_ERROR:
            ESP_LOGE(TAG, "MQTT error event");
            break;
        default:
            break;
    }
}

void app_main(void)
{
    esp_err_t ret = nvs_flash_init();
    if (ret == ESP_ERR_NVS_NO_FREE_PAGES || ret == ESP_ERR_NVS_NEW_VERSION_FOUND) {
        ESP_ERROR_CHECK(nvs_flash_erase());
        ret = nvs_flash_init();
    }
    ESP_ERROR_CHECK(ret);

    ota_validate_image();

    led_init();
    led_set_color(0, 0, 64);

    const char *wifi_ssid = runtime_get_wifi_ssid();
    const char *wifi_password = runtime_get_wifi_password();
    const char *mqtt_broker_uri = runtime_get_mqtt_broker_uri();
    const char *device_id = runtime_get_device_id();

    strncpy(s_device_id, device_id, sizeof(s_device_id) - 1);
    s_device_id[sizeof(s_device_id) - 1] = '\0';
    if (s_device_id[0] == '\0') {
        strncpy(s_device_id, CONFIG_DEVICE_ID, sizeof(s_device_id) - 1);
        s_device_id[sizeof(s_device_id) - 1] = '\0';
    }

    init_template_runtime();

    if (runtime_config_has_overrides()) {
        ESP_LOGI(TAG, "Using flash-time network configuration override");
    }
    ESP_LOGI(TAG, "Wi-Fi SSID: %s", wifi_ssid);
    ESP_LOGI(TAG, "MQTT broker: %s", mqtt_broker_uri);

    ret = wifi_station_init(wifi_ssid, wifi_password);
    if (ret != ESP_OK) {
        ESP_LOGE(TAG, "WiFi connection failed, LED red");
        led_set_color(64, 0, 0);
        return;
    }

    led_set_color(0, 64, 0);
    vTaskDelay(pdMS_TO_TICKS(500));

    snprintf(s_topic_state, sizeof(s_topic_state), "sol/devices/%s/state", s_device_id);
    snprintf(s_topic_cmd, sizeof(s_topic_cmd), "sol/devices/%s/cmd", s_device_id);
    snprintf(s_topic_ack, sizeof(s_topic_ack), "sol/devices/%s/ack", s_device_id);

    snprintf(s_lwt_payload, sizeof(s_lwt_payload),
             "{\"deviceId\":\"%s\",\"name\":\"ESP32 LED Controller\",\"online\":false,\"ts\":0}",
             s_device_id);

    esp_mqtt_client_config_t mqtt_cfg = {
        .broker.address.uri = mqtt_broker_uri,
        .session.protocol_ver = MQTT_PROTOCOL_V_5,
        .session.keepalive = 30,
        .credentials.client_id = s_device_id,
        .session.last_will.topic = s_topic_state,
        .session.last_will.msg = s_lwt_payload,
        .session.last_will.msg_len = 0,
        .session.last_will.qos = 1,
        .session.last_will.retain = true,
        .buffer.size = 8192,
    };

    s_mqtt_client = esp_mqtt_client_init(&mqtt_cfg);
    if (!s_mqtt_client) {
        ESP_LOGE(TAG, "Failed to init MQTT client");
        led_set_color(64, 0, 0);
        return;
    }

    ESP_ERROR_CHECK(esp_mqtt_client_register_event(s_mqtt_client, ESP_EVENT_ANY_ID, mqtt_event_handler, NULL));
    ESP_LOGI(TAG, "Starting MQTT client...");
    ESP_ERROR_CHECK(esp_mqtt_client_start(s_mqtt_client));

    if (s_template_mode == RUNTIME_TEMPLATE_ENV_SENSOR) {
        env_sensor_start(s_mqtt_client, s_device_id);
    } else if (s_template_mode == RUNTIME_TEMPLATE_SMART_PLUG) {
        smart_plug_start(s_mqtt_client, s_device_id);
    }

    led_off();
    ESP_LOGI(TAG, "Device online, waiting for MQTT commands...");

    while (1) {
        if (s_mqtt_connected) {
            TickType_t now = xTaskGetTickCount();
            if ((now - s_last_state_publish_tick) >= pdMS_TO_TICKS(STATE_HEARTBEAT_INTERVAL_MS)) {
                publish_state(true);
                s_last_state_publish_tick = now;
            }
        }
        vTaskDelay(pdMS_TO_TICKS(1000));
    }
}
