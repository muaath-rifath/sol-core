#include <stdio.h>
#include <string.h>

#include "esp_log.h"
#include "esp_system.h"
#include "esp_event.h"
#include "esp_netif.h"
#include "esp_tls.h"
#include "mqtt_client.h"
#include "nvs_flash.h"
#include "cJSON.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/event_groups.h"
#include "driver/gpio.h"
#include "ota.h"
#include "certs.h"
#include "device_driver.h"
#include "esp_crt_bundle.h"
#include "runtime_config.h"
#include "wifi.h"

static const char *TAG = "main";

static esp_mqtt_client_handle_t s_mqtt_client = NULL;

static char s_topic_state[128];
static char s_topic_cmd[128];
static char s_topic_ack[128];
static char s_topic_ota[128];
static char s_lwt_payload[160];
static char s_device_id[RUNTIME_DEVICE_ID_MAX_LEN + 1];
static char s_last_ota_request_id[80];
static volatile bool s_ota_cancel_requested = false;
static volatile bool s_ota_in_progress = false;
static TickType_t s_last_state_publish_tick = 0;
static bool s_mqtt_connected = false;
static const device_driver_t *s_active_driver = NULL;

#define STATE_HEARTBEAT_INTERVAL_MS 2000

static cert_bundle_t s_certs = {0};



static bool ota_cancel_requested_cb(void) { return s_ota_cancel_requested; }

static void publish_ota_status(const char *status, int progress,
                               const char *message, const char *error) {
  if (!s_mqtt_client || s_last_ota_request_id[0] == '\0') {
    return;
  }

  cJSON *root = cJSON_CreateObject();
  if (!root) {
    return;
  }

  cJSON_AddStringToObject(root, "requestId", s_last_ota_request_id);
  cJSON_AddStringToObject(root, "status", status ? status : "downloading");
  cJSON_AddNumberToObject(root, "progress", progress);
  if (message) {
    cJSON_AddStringToObject(root, "message", message);
    cJSON_AddStringToObject(root, "log", message);
  }
  if (error && error[0] != '\0') {
    cJSON_AddStringToObject(root, "error", error);
  }
  cJSON_AddNumberToObject(root, "ts", (double)esp_log_timestamp());

  char *payload = cJSON_PrintUnformatted(root);
  cJSON_Delete(root);
  if (!payload) {
    return;
  }

  int msg_id =
      esp_mqtt_client_publish(s_mqtt_client, s_topic_ota, payload, 0, 1, 0);
  if (msg_id < 0) {
    ESP_LOGW(TAG, "Failed to publish OTA status");
  }
  cJSON_free(payload);
}

static void publish_state(bool online) {
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
  cJSON_AddNumberToObject(state, "ts", (double)esp_log_timestamp());
  cJSON_AddStringToObject(state, "ip_address", wifi_get_ip());

  if (s_active_driver && s_active_driver->get_state) {
    s_active_driver->get_state(state);
  }

  char *payload = cJSON_PrintUnformatted(state);
  cJSON_Delete(state);
  if (!payload) {
    return;
  }

  int msg_id =
      esp_mqtt_client_publish(s_mqtt_client, s_topic_state, payload, 0, 1, 1);
  if (msg_id < 0) {
    ESP_LOGW(TAG, "Failed to publish %s state", online ? "online" : "offline");
  }
  cJSON_free(payload);
}



static void publish_ack(const char *request_id, bool ok, const char *message) {
  if (!s_mqtt_client || !request_id) {
    return;
  }

  cJSON *root = cJSON_CreateObject();
  if (!root) {
    return;
  }

  cJSON_AddStringToObject(root, "requestId", request_id);
  cJSON_AddBoolToObject(root, "ok", ok);
  cJSON_AddStringToObject(root, "message",
                          message ? message : (ok ? "OK" : "Error"));
  cJSON_AddNumberToObject(root, "ts", (double)esp_log_timestamp());

  char *payload = cJSON_PrintUnformatted(root);
  cJSON_Delete(root);
  if (!payload) {
    return;
  }

  int msg_id =
      esp_mqtt_client_publish(s_mqtt_client, s_topic_ack, payload, 0, 1, 0);
  if (msg_id < 0) {
    ESP_LOGW(TAG, "Failed to publish ack");
  }
  cJSON_free(payload);
}

static void ota_task(void *param) {
  char *url = (char *)param;
  ESP_LOGI(TAG, "ota_task started");
  if (!url) {
    vTaskDelete(NULL);
    return;
  }

  s_ota_in_progress = true;
  s_ota_cancel_requested = false;

  publish_ota_status("initiated", 1, "ota_task starting ota_start", NULL);
  esp_err_t ret = ota_start(url, s_certs.client_cert, s_certs.client_key, publish_ota_status, ota_cancel_requested_cb);
  if (ret != ESP_OK && !s_ota_cancel_requested) {
    ESP_LOGE(TAG, "OTA failed: %s", esp_err_to_name(ret));
    publish_ota_status("failed", 0, "OTA failed", esp_err_to_name(ret));
  } else if (s_ota_cancel_requested) {
    publish_ota_status("cancelled", 0, "OTA cancelled by user", NULL);
  }

  s_ota_in_progress = false;
  s_ota_cancel_requested = false;
  free(url);
  vTaskDelete(NULL);
}

static void handle_command_json(const char *json_text) {
  if (!json_text) {
    return;
  }

  cJSON *root = cJSON_Parse(json_text);
  if (!root) {
    ESP_LOGW(TAG, "Invalid JSON command payload");
    return;
  }

  const cJSON *request_id_obj =
      cJSON_GetObjectItemCaseSensitive(root, "requestId");
  const char *req_id =
      cJSON_IsString(request_id_obj) ? request_id_obj->valuestring : "internal";

  const cJSON *action_obj = cJSON_GetObjectItemCaseSensitive(root, "action");
  const cJSON *params_obj = cJSON_GetObjectItemCaseSensitive(root, "params");

  // Compatibility check for old format: {"cmd": "...", "args": {...}}
  const cJSON *cmd_obj = cJSON_GetObjectItemCaseSensitive(root, "cmd");
  const cJSON *args_obj = cJSON_GetObjectItemCaseSensitive(root, "args");

  const char *action =
      cJSON_IsString(action_obj)
          ? action_obj->valuestring
          : (cJSON_IsString(cmd_obj) ? cmd_obj->valuestring : NULL);
  const cJSON *params = cJSON_IsObject(params_obj) ? params_obj : args_obj;

  if (!action) {
    cJSON_Delete(root);
    return;
  }

  if (strcmp(action, "ota_update") == 0) {
    const cJSON *url_obj = cJSON_GetObjectItemCaseSensitive(params, "url");
    const cJSON *req_id_obj =
        cJSON_GetObjectItemCaseSensitive(params, "request_id");
    if (cJSON_IsString(req_id_obj) && req_id_obj->valuestring[0] != '\0') {
      strncpy(s_last_ota_request_id, req_id_obj->valuestring,
              sizeof(s_last_ota_request_id) - 1);
      s_last_ota_request_id[sizeof(s_last_ota_request_id) - 1] = '\0';
    } else {
      strncpy(s_last_ota_request_id, req_id, sizeof(s_last_ota_request_id) - 1);
      s_last_ota_request_id[sizeof(s_last_ota_request_id) - 1] = '\0';
    }
    if (cJSON_IsString(url_obj)) {
      if (s_ota_in_progress) {
        publish_ack(s_last_ota_request_id, false, "OTA already in progress");
        publish_ota_status("failed", 0, "OTA already in progress", "busy");
        cJSON_Delete(root);
        return;
      }
      char *url = strdup(url_obj->valuestring);
      publish_ack(s_last_ota_request_id, true, "OTA update started");
      publish_ota_status("acknowledged", 3, "OTA command acknowledged", NULL);
      xTaskCreate(ota_task, "ota_task", 8192, url, 5, NULL);
    } else {
      publish_ack(s_last_ota_request_id, false, "missing OTA URL");
      publish_ota_status("failed", 0, "missing OTA URL", "missing OTA URL");
    }
  } else if (strcmp(action, "ota_cancel") == 0) {
    const cJSON *req_id_obj =
        cJSON_GetObjectItemCaseSensitive(params, "request_id");
    const char *cancel_id =
        cJSON_IsString(req_id_obj) ? req_id_obj->valuestring : req_id;

    if (cancel_id && cancel_id[0] != '\0' && s_last_ota_request_id[0] != '\0' &&
        strcmp(cancel_id, s_last_ota_request_id) != 0) {
      publish_ack(cancel_id, false, "request id does not match active OTA");
      publish_ota_status("failed", 0, "cancel rejected: request id mismatch",
                         "request id mismatch");
      cJSON_Delete(root);
      return;
    }

    if (!s_ota_in_progress) {
      publish_ack(cancel_id, false, "no active OTA in progress");
      publish_ota_status("failed", 0, "no active OTA in progress", "idle");
      cJSON_Delete(root);
      return;
    }

    s_ota_cancel_requested = true;
    publish_ack(cancel_id, true, "OTA cancellation requested");
    publish_ota_status("cancelling", 0, "OTA cancellation requested", NULL);
  } else {
    if (s_active_driver && s_active_driver->handle_mqtt) {
        s_active_driver->handle_mqtt(action, params, req_id);
        publish_ack(req_id, true, "command forwarded to device driver");
        publish_state(true);
    } else {
        publish_ack(req_id, false, "unknown command or no driver");
    }
  }
  cJSON_Delete(root);
}

static void mqtt_event_handler(void *handler_args, esp_event_base_t base,
                               int32_t event_id, void *event_data) {
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
    if (!event->topic || event->topic_len <= 0 || !event->data ||
        event->data_len <= 0) {
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

#include "esp_sntp.h"
#include <time.h>
#include <sys/time.h>

static void initialize_sntp(void) {
  ESP_LOGI(TAG, "Initializing SNTP");
  esp_sntp_setoperatingmode(SNTP_OPMODE_POLL);
  esp_sntp_setservername(0, "pool.ntp.org");
  esp_sntp_init();
  
  // Wait for time to be set
  int retry = 0;
  const int retry_count = 15;
  while (sntp_get_sync_status() == SNTP_SYNC_STATUS_RESET && ++retry < retry_count) {
    ESP_LOGI(TAG, "Waiting for system time to be set... (%d/%d)", retry, retry_count);
    vTaskDelay(pdMS_TO_TICKS(2000));
  }
  
  time_t now = 0;
  struct tm timeinfo = { 0 };
  time(&now);
  localtime_r(&now, &timeinfo);
  if (timeinfo.tm_year < (2020 - 1900)) {
    ESP_LOGE(TAG, "Failed to get current time from NTP server.");
  } else {
    ESP_LOGI(TAG, "System time is set: %s", asctime(&timeinfo));
  }
}

void app_main(void) {
  esp_err_t ret = nvs_flash_init();
  if (ret == ESP_ERR_NVS_NO_FREE_PAGES ||
      ret == ESP_ERR_NVS_NEW_VERSION_FOUND) {
    ESP_ERROR_CHECK(nvs_flash_erase());
    ret = nvs_flash_init();
  }
  ESP_ERROR_CHECK(ret);

  ota_validate_image();

  s_active_driver = device_driver_find(runtime_get_template_id());
  if (s_active_driver && s_active_driver->init) {
      s_active_driver->init();
  }

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



  if (runtime_config_has_overrides()) {
    ESP_LOGI(TAG, "Using flash-time network configuration override");
  }
  ESP_LOGI(TAG, "Wi-Fi SSID: %s", wifi_ssid);
  ESP_LOGI(TAG, "MQTT broker: %s", mqtt_broker_uri);

  ret = wifi_station_init(wifi_ssid, wifi_password);
  if (ret != ESP_OK) {
    ESP_LOGE(TAG, "WiFi connection failed");
    return;
  }

  initialize_sntp();


  vTaskDelay(pdMS_TO_TICKS(500));

  snprintf(s_topic_state, sizeof(s_topic_state), "sol/devices/%s/state",
           s_device_id);
  snprintf(s_topic_cmd, sizeof(s_topic_cmd), "sol/devices/%s/cmd", s_device_id);
  snprintf(s_topic_ack, sizeof(s_topic_ack), "sol/devices/%s/ack", s_device_id);
  snprintf(s_topic_ota, sizeof(s_topic_ota), "sol/devices/%s/ota", s_device_id);

  snprintf(s_lwt_payload, sizeof(s_lwt_payload),
           "{\"deviceId\":\"%s\",\"name\":\"ESP32 LED "
           "Controller\",\"online\":false,\"ts\":0}",
           s_device_id);

  s_certs = (cert_bundle_t){0};
  bool use_mtls = (certs_load(&s_certs) == ESP_OK);

  esp_mqtt_client_config_t mqtt_cfg = {
      .broker.address.uri = mqtt_broker_uri,
      .broker.verification.crt_bundle_attach = esp_crt_bundle_attach,
      .broker.verification.skip_cert_common_name_check = false,


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

  if (use_mtls) {
    ESP_LOGI(TAG, "Using mTLS certificates from flash");
    mqtt_cfg.credentials.authentication.certificate = s_certs.client_cert;
    mqtt_cfg.credentials.authentication.key = s_certs.client_key;
  } else {
    ESP_LOGI(TAG, "Using MQTT password authentication");
    mqtt_cfg.credentials.username = runtime_get_mqtt_username();
    mqtt_cfg.credentials.authentication.password = runtime_get_mqtt_password();
  }

  s_mqtt_client = esp_mqtt_client_init(&mqtt_cfg);
  if (!s_mqtt_client) {
    ESP_LOGE(TAG, "Failed to init MQTT client");
    certs_free(&s_certs);
    return;
  }

  ESP_ERROR_CHECK(esp_mqtt_client_register_event(
      s_mqtt_client, ESP_EVENT_ANY_ID, mqtt_event_handler, NULL));
  ESP_LOGI(TAG, "Starting MQTT client...");
  ESP_ERROR_CHECK(esp_mqtt_client_start(s_mqtt_client));

  // Note: We don't call certs_free here because esp_mqtt_client_init might use
  // the pointers. However, in standard ESP-IDF mqtt, the client makes a copy if
  // configured to do so. To be safe, we leak this memory for now as it's a
  // one-time allocation.


  ESP_LOGI(TAG, "Device online, waiting for MQTT commands...");

  while (1) {
    if (s_mqtt_connected) {
      TickType_t now = xTaskGetTickCount();
      if ((now - s_last_state_publish_tick) >=
          pdMS_TO_TICKS(STATE_HEARTBEAT_INTERVAL_MS)) {
        publish_state(true);
        s_last_state_publish_tick = now;
      }
    }
    vTaskDelay(pdMS_TO_TICKS(1000));
  }
}
