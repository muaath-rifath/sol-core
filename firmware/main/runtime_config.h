#pragma once

#include <stdbool.h>
#include <stdint.h>

#define RUNTIME_WIFI_SSID_MAX_LEN 32
#define RUNTIME_WIFI_PASSWORD_MAX_LEN 64
#define RUNTIME_MQTT_URI_MAX_LEN 128
#define RUNTIME_DEVICE_ID_MAX_LEN 32
#define RUNTIME_TEMPLATE_ID_MAX_LEN 32

#define RUNTIME_RELAY_CHANNELS_MAX 4

typedef enum {
    RUNTIME_TEMPLATE_RGB_LED = 0,
    RUNTIME_TEMPLATE_RELAY_SINGLE = 1,
    RUNTIME_TEMPLATE_RELAY_4CH_GPIO = 2,
    RUNTIME_TEMPLATE_ENV_SENSOR = 3,
    RUNTIME_TEMPLATE_SMART_PLUG = 4,
} runtime_template_mode_t;

const char *runtime_get_wifi_ssid(void);
const char *runtime_get_wifi_password(void);
const char *runtime_get_mqtt_broker_uri(void);
const char *runtime_get_device_id(void);
const char *runtime_get_template_id(void);
runtime_template_mode_t runtime_get_template_mode(void);
int runtime_get_relay_pin(uint8_t channel_index);
bool runtime_is_relay_active_low(uint8_t channel_index);

// Returns true when at least one value was overridden by a flash-time patch.
bool runtime_config_has_overrides(void);
