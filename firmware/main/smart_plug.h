#pragma once

#include "mqtt_client.h"
#include <stdbool.h>

void smart_plug_start(esp_mqtt_client_handle_t mqtt_client, const char *device_id);
void smart_plug_set_state(bool is_on);
