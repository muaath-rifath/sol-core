#pragma once

#include "mqtt_client.h"

void env_sensor_start(esp_mqtt_client_handle_t mqtt_client, const char *device_id);
