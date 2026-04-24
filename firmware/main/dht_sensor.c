#include "dht_sensor.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_log.h"
#include "esp_random.h"
#include "cJSON.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

static const char *TAG = "env_sensor";
static esp_mqtt_client_handle_t s_mqtt = NULL;
static char s_telemetry_topic[128];

static void env_sensor_task(void *param) {
    while (1) {
        vTaskDelay(pdMS_TO_TICKS(10000));
        
        if (!s_mqtt) continue;

        cJSON *root = cJSON_CreateObject();
        if (!root) continue;
        
        float temp = 22.0 + ((esp_random() % 500) / 100.0); // 22.0 to 26.99 C
        float humid = 40.0 + ((esp_random() % 2000) / 100.0); // 40.0 to 59.99 %
        
        cJSON_AddNumberToObject(root, "temperature", temp);
        cJSON_AddNumberToObject(root, "humidity", humid);

        char *payload = cJSON_PrintUnformatted(root);
        cJSON_Delete(root);
        if (payload) {
            esp_mqtt_client_publish(s_mqtt, s_telemetry_topic, payload, 0, 0, 0);
            free(payload);
        }
    }
}

void env_sensor_start(esp_mqtt_client_handle_t mqtt_client, const char *device_id) {
    s_mqtt = mqtt_client;
    snprintf(s_telemetry_topic, sizeof(s_telemetry_topic), "sol/devices/%s/telemetry", device_id);
    xTaskCreate(env_sensor_task, "env_sensor_task", 4096, NULL, 5, NULL);
    ESP_LOGI(TAG, "Started Env Sensor Simulation Task");
}
