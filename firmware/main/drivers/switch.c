#include "device_driver.h"
#include "runtime_config.h"
#include "esp_log.h"
#include "driver/gpio.h"
#include "cJSON.h"
#include <string.h>

static const char *TAG = "driver_switch";

static int s_relay_pins[RUNTIME_RELAY_CHANNELS_MAX] = {-1, -1, -1, -1};
static bool s_relay_logical_states[RUNTIME_RELAY_CHANNELS_MAX] = {false, false, false, false};

static bool relay_channel_valid(uint8_t channel) {
    return channel < RUNTIME_RELAY_CHANNELS_MAX && s_relay_pins[channel] >= 0;
}

static esp_err_t relay_write_channel(uint8_t channel, bool logical_on) {
    if (!relay_channel_valid(channel)) {
        return ESP_ERR_INVALID_ARG;
    }

    bool active_low = runtime_is_relay_active_low(channel);
    int level = logical_on ? (active_low ? 0 : 1) : (active_low ? 1 : 0);

    s_relay_logical_states[channel] = logical_on;
    return gpio_set_level((gpio_num_t)s_relay_pins[channel], level);
}

static void switch_init(void) {
    ESP_LOGI(TAG, "Initializing switch driver");
    
    for (uint8_t i = 0; i < RUNTIME_RELAY_CHANNELS_MAX; i++) {
        s_relay_pins[i] = runtime_get_relay_pin(i);
        if (s_relay_pins[i] < 0) {
            continue;
        }
        
        gpio_reset_pin((gpio_num_t)s_relay_pins[i]);
        gpio_set_direction((gpio_num_t)s_relay_pins[i], GPIO_MODE_OUTPUT);
        
        // Initialize to logical off
        relay_write_channel(i, false);
        ESP_LOGI(TAG, "Initialized relay channel %d on GPIO %d", i, s_relay_pins[i]);
    }
}

static void switch_handle_mqtt(const cJSON *payload, const char *req_id) {
    cJSON *relays = cJSON_GetObjectItem(payload, "relays");
    if (cJSON_IsArray(relays)) {
        int i = 0;
        cJSON *relay = NULL;
        cJSON_ArrayForEach(relay, relays) {
            if (i >= RUNTIME_RELAY_CHANNELS_MAX) break;
            
            if (cJSON_IsBool(relay) && relay_channel_valid(i)) {
                bool on = cJSON_IsTrue(relay);
                relay_write_channel(i, on);
                ESP_LOGI(TAG, "Set relay %d to %s", i, on ? "ON" : "OFF");
            }
            i++;
        }
    }
}

static void switch_get_state(cJSON *state) {
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

const device_driver_t s_switch_driver = {
    .template_id = "switch",
    .init = switch_init,
    .handle_mqtt = switch_handle_mqtt,
    .get_state = switch_get_state
};
