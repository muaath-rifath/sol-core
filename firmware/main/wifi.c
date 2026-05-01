#include "wifi.h"

#include <stdint.h>
#include <string.h>
#include "freertos/FreeRTOS.h"
#include "freertos/event_groups.h"
#include "freertos/task.h"
#include "esp_wifi.h"
#include "esp_event.h"
#include "esp_log.h"
#include "esp_netif.h"

#define WIFI_CONNECTED_BIT BIT0
#define WIFI_FAIL_BIT      BIT1
#define MAX_RETRY                  12
#define WIFI_CONNECT_TIMEOUT_MS    90000
#define RETRY_BACKOFF_INITIAL_MS   500
#define RETRY_BACKOFF_MAX_MS       10000

static const char *TAG = "wifi";
static EventGroupHandle_t s_wifi_event_group;
static int s_retry_num = 0;
static bool s_startup_connected = false;
static int s_retry_backoff_ms = RETRY_BACKOFF_INITIAL_MS;
static bool s_reconnect_task_active = false;
static char s_ip_address[16] = "0.0.0.0";

const char* wifi_get_ip(void)
{
    return s_ip_address;
}

static const char *disconnect_reason_to_str(wifi_err_reason_t reason)
{
    switch (reason) {
        case WIFI_REASON_AUTH_EXPIRE:
            return "AUTH_EXPIRE";
        case WIFI_REASON_AUTH_FAIL:
            return "AUTH_FAIL";
        case WIFI_REASON_ASSOC_FAIL:
            return "ASSOC_FAIL";
        case WIFI_REASON_HANDSHAKE_TIMEOUT:
            return "HANDSHAKE_TIMEOUT";
        case WIFI_REASON_4WAY_HANDSHAKE_TIMEOUT:
            return "4WAY_HANDSHAKE_TIMEOUT";
        case WIFI_REASON_NO_AP_FOUND:
            return "NO_AP_FOUND";
        case WIFI_REASON_CONNECTION_FAIL:
            return "CONNECTION_FAIL";
        default:
            return "UNKNOWN";
    }
}

static bool is_non_retriable_reason(wifi_err_reason_t reason)
{
    return reason == WIFI_REASON_AUTH_FAIL;
}

static void delayed_reconnect_task(void *arg)
{
    int delay_ms = (int)(intptr_t)arg;
    if (delay_ms > 0) {
        vTaskDelay(pdMS_TO_TICKS(delay_ms));
    }

    ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());
    s_reconnect_task_active = false;
    vTaskDelete(NULL);
}

static bool schedule_reconnect(int delay_ms)
{
    if (s_reconnect_task_active) {
        return false;
    }

    s_reconnect_task_active = true;
    BaseType_t ok = xTaskCreate(
        delayed_reconnect_task,
        "wifi_reconnect",
        3072,
        (void *)(intptr_t)delay_ms,
        tskIDLE_PRIORITY + 1,
        NULL);

    if (ok != pdPASS) {
        s_reconnect_task_active = false;
        ESP_LOGW(TAG, "Reconnect task create failed, trying immediate reconnect");
        ESP_ERROR_CHECK_WITHOUT_ABORT(esp_wifi_connect());
        return true;
    }

    return true;
}

static void event_handler(void *arg, esp_event_base_t event_base,
                           int32_t event_id, void *event_data)
{
    if (event_base == WIFI_EVENT && event_id == WIFI_EVENT_STA_START) {
        schedule_reconnect(0);
    } else if (event_base == WIFI_EVENT && event_id == WIFI_EVENT_STA_DISCONNECTED) {
        wifi_event_sta_disconnected_t *disconn = (wifi_event_sta_disconnected_t *)event_data;
        wifi_err_reason_t reason = disconn ? disconn->reason : WIFI_REASON_UNSPECIFIED;
        ESP_LOGW(TAG, "Disconnected (reason=%d:%s)", reason, disconnect_reason_to_str(reason));

        if (!s_startup_connected && is_non_retriable_reason(reason)) {
            ESP_LOGE(TAG, "Non-retriable startup Wi-Fi failure (%s)", disconnect_reason_to_str(reason));
            xEventGroupSetBits(s_wifi_event_group, WIFI_FAIL_BIT);
            return;
        }

        if (!s_startup_connected && s_retry_num < MAX_RETRY) {
            int delay_ms = s_retry_backoff_ms;
            if (schedule_reconnect(delay_ms)) {
                s_retry_num++;
                if (s_retry_backoff_ms < RETRY_BACKOFF_MAX_MS) {
                    s_retry_backoff_ms *= 2;
                    if (s_retry_backoff_ms > RETRY_BACKOFF_MAX_MS) {
                        s_retry_backoff_ms = RETRY_BACKOFF_MAX_MS;
                    }
                }
                ESP_LOGI(TAG, "Retrying connection (%d/%d) in %d ms", s_retry_num, MAX_RETRY, delay_ms);
            } else {
                ESP_LOGD(TAG, "Reconnect already pending, ignoring duplicate disconnect event");
            }
        } else if (!s_startup_connected) {
            // Exhausted initial retries — reset and keep trying forever.
            ESP_LOGW(TAG, "Initial Wi-Fi connection failed after %d retries, resetting and retrying...", MAX_RETRY);
            s_retry_num = 0;
            s_retry_backoff_ms = RETRY_BACKOFF_INITIAL_MS;
            schedule_reconnect(RETRY_BACKOFF_MAX_MS);
        } else {
            // After first successful connection, keep trying forever to recover Wi-Fi.
            if (schedule_reconnect(RETRY_BACKOFF_INITIAL_MS)) {
                ESP_LOGI(TAG, "Reconnecting Wi-Fi in %d ms...", RETRY_BACKOFF_INITIAL_MS);
            }
        }
    } else if (event_base == IP_EVENT && event_id == IP_EVENT_STA_GOT_IP) {
        ip_event_got_ip_t *event = (ip_event_got_ip_t *)event_data;
        snprintf(s_ip_address, sizeof(s_ip_address), IPSTR, IP2STR(&event->ip_info.ip));
        ESP_LOGI(TAG, "Connected, IP: %s", s_ip_address);
        s_startup_connected = true;
        s_retry_num = 0;
        s_retry_backoff_ms = RETRY_BACKOFF_INITIAL_MS;
        xEventGroupSetBits(s_wifi_event_group, WIFI_CONNECTED_BIT);
    }
}

esp_err_t wifi_station_init(const char *ssid, const char *password)
{
    s_wifi_event_group = xEventGroupCreate();
    s_retry_num = 0;
    s_startup_connected = false;
    s_retry_backoff_ms = RETRY_BACKOFF_INITIAL_MS;
    s_reconnect_task_active = false;

    ESP_ERROR_CHECK(esp_netif_init());
    ESP_ERROR_CHECK(esp_event_loop_create_default());
    esp_netif_create_default_wifi_sta();

    wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();
    ESP_ERROR_CHECK(esp_wifi_init(&cfg));

    esp_event_handler_instance_t instance_any_id;
    esp_event_handler_instance_t instance_got_ip;
    ESP_ERROR_CHECK(esp_event_handler_instance_register(
        WIFI_EVENT, ESP_EVENT_ANY_ID, &event_handler, NULL, &instance_any_id));
    ESP_ERROR_CHECK(esp_event_handler_instance_register(
        IP_EVENT, IP_EVENT_STA_GOT_IP, &event_handler, NULL, &instance_got_ip));

    ESP_ERROR_CHECK(esp_wifi_set_mode(WIFI_MODE_STA));

    if (ssid != NULL && strlen(ssid) > 0) {
        wifi_config_t wifi_config = {
            .sta = {
                .threshold.authmode = WIFI_AUTH_WPA_WPA2_PSK,
                .pmf_cfg = {
                    .capable = true,
                    .required = false,
                },
            },
        };
        strncpy((char *)wifi_config.sta.ssid, ssid, sizeof(wifi_config.sta.ssid) - 1);
        if (password != NULL) {
            strncpy((char *)wifi_config.sta.password, password, sizeof(wifi_config.sta.password) - 1);
        }
        ESP_ERROR_CHECK(esp_wifi_set_config(WIFI_IF_STA, &wifi_config));
        ESP_LOGI(TAG, "Connecting to configured SSID: %s...", ssid);
    } else {
        ESP_LOGI(TAG, "No SSID configured, using previously saved credentials from NVS...");
    }

    ESP_ERROR_CHECK(esp_wifi_start());
    ESP_ERROR_CHECK(esp_wifi_set_ps(WIFI_PS_NONE));

    	// Wait indefinitely — the event handler retries forever, so we only
	// get WIFI_FAIL_BIT on a non-recoverable error (e.g. AUTH_FAIL).
	EventBits_t bits = xEventGroupWaitBits(s_wifi_event_group,
		WIFI_CONNECTED_BIT | WIFI_FAIL_BIT,
		pdFALSE, pdFALSE, portMAX_DELAY);

	if (bits & WIFI_CONNECTED_BIT) {
		ESP_LOGI(TAG, "Connected to %s", ssid);
		return ESP_OK;
	}

	ESP_LOGE(TAG, "Failed to connect to %s (non-retriable error)", ssid);
	return ESP_FAIL;
}
