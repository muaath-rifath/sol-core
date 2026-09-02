#pragma once
#include "mqtt_client.h"

/*
 * voice_init — initialise wake word detection and LiveKit subsystems.
 * Call once after the MQTT client has been started.
 */
void voice_init(esp_mqtt_client_handle_t client, const char *device_id);

/*
 * voice_handle_session — handle the JSON payload received on
 * sol/devices/{id}/voice.  Call from the MQTT_EVENT_DATA handler.
 * payload must be a null-terminated string.
 */
void voice_handle_session(const char *payload);
