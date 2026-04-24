#include "smart_plug.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"
#include "esp_random.h"
#include "cJSON.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static const char *TAG = "smart_plug";
static esp_mqtt_client_handle_t s_mqtt = NULL;
static char s_telemetry_topic[128];
static bool s_is_on = false;

static void smart_plug_task(void *param) {
    while (1) {
        vTaskDelay(pdMS_TO_TICKS(5000));
        
        if (!s_mqtt) continue;

        cJSON *root = cJSON_CreateObject();
        if (!root) continue;
        
        float power = s_is_on ? (1200.0 + ((esp_random() % 500) / 100.0)) : 0.0;
        
        cJSON_AddNumberToObject(root, "power_w", power);

        char *payload = cJSON_PrintUnformatted(root);
        cJSON_Delete(root);
        if (payload) {
            esp_mqtt_client_publish(s_mqtt, s_telemetry_topic, payload, 0, 0, 0);
            free(payload);
        }
    }
}

void smart_plug_start(esp_mqtt_client_handle_t mqtt_client, const char *device_id) {
    s_mqtt = mqtt_client;
    snprintf(s_telemetry_topic, sizeof(s_telemetry_topic), "sol/devices/%s/telemetry", device_id);
    xTaskCreate(smart_plug_task, "smart_plug_task", 4096, NULL, 5, NULL);
    ESP_LOGI(TAG, "Started Smart Plug Telemetry Task");
}

void smart_plug_set_state(bool is_on) {
    s_is_on = is_on;
}
