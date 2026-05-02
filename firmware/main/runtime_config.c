#include "runtime_config.h"

#include <stdbool.h>
#include <string.h>
#include <stdlib.h>

#include "esp_log.h"
#include "esp_partition.h"
#include "nvs.h"

#define TAG "runtime_cfg"
#define NVS_NAMESPACE "sol_cfg"

#define PATCH_SIGNATURE_V1 "SOLCFGv1::ESP32"
#define PATCH_SIGNATURE_V2 "SOLCFGv2::ESP32"
#define PATCH_SIGNATURE_FIELD_SIZE 16

typedef struct {
    char signature[PATCH_SIGNATURE_FIELD_SIZE];
    char wifi_ssid[RUNTIME_WIFI_SSID_MAX_LEN + 1];
    char wifi_password[RUNTIME_WIFI_PASSWORD_MAX_LEN + 1];
    char mqtt_uri[RUNTIME_MQTT_URI_MAX_LEN + 1];
} runtime_config_blob_v1_t;

typedef struct {
    char signature[PATCH_SIGNATURE_FIELD_SIZE];
    char wifi_ssid[RUNTIME_WIFI_SSID_MAX_LEN + 1];
    char wifi_password[RUNTIME_WIFI_PASSWORD_MAX_LEN + 1];
    char mqtt_uri[RUNTIME_MQTT_URI_MAX_LEN + 1];
    char mqtt_username[RUNTIME_MQTT_USERNAME_MAX_LEN + 1];
    char mqtt_password[RUNTIME_MQTT_PASSWORD_MAX_LEN + 1];
    char device_id[RUNTIME_DEVICE_ID_MAX_LEN + 1];
    char template_id[RUNTIME_TEMPLATE_ID_MAX_LEN + 1];
    uint8_t relay_pins[RUNTIME_RELAY_CHANNELS_MAX];
    uint8_t relay_active_low_mask;
    uint8_t reserved[11];
} runtime_config_blob_v2_t;

// Patched in-place by web flasher before writing app flash.
// Must NOT be const.
__attribute__((used)) static runtime_config_blob_v2_t RUNTIME_CONFIG_BLOB = {
    .signature = PATCH_SIGNATURE_V2,
    .wifi_ssid = "",
    .wifi_password = "",
    .mqtt_uri = "",
    .mqtt_username = "",
    .mqtt_password = "",
    .device_id = "",
    .template_id = "",
    .relay_pins = {0, 0, 0, 0},
    .relay_active_low_mask = 0,
    .reserved = {0},
};

static bool runtime_blob_has_signature(const char *signature)
{
    size_t sig_len = strlen(signature);
    return strncmp(RUNTIME_CONFIG_BLOB.signature, signature, sig_len) == 0;
}

static bool runtime_blob_valid_v2(void)
{
    return runtime_blob_has_signature(PATCH_SIGNATURE_V2);
}

static bool runtime_blob_valid_v1(void)
{
    return runtime_blob_has_signature(PATCH_SIGNATURE_V1);
}

static bool copy_runtime_or_default(char *out, size_t out_size,
                                    const char *runtime_value,
                                    const char *default_value,
                                    bool runtime_valid)
{
    const char *source = runtime_value;
    bool has_override = false;

    if (!runtime_valid || runtime_value[0] == '\0') {
        source = default_value;
    } else {
        has_override = true;
    }

    strncpy(out, source, out_size - 1);
    out[out_size - 1] = '\0';
    return has_override;
}

// ---------------------------------------------------------------------------
// NVS persistence — survives OTA because NVS is a separate flash partition
// ---------------------------------------------------------------------------

static bool load_from_nvs(void)
{
    nvs_handle_t h;
    if (nvs_open(NVS_NAMESPACE, NVS_READONLY, &h) != ESP_OK) {
        return false;
    }

    // Use device_id as the presence sentinel — it's the most critical field.
    char device_id[RUNTIME_DEVICE_ID_MAX_LEN + 1] = {0};
    size_t len = sizeof(device_id);
    bool found = (nvs_get_str(h, "device_id", device_id, &len) == ESP_OK && device_id[0] != '\0');

    if (found) {
        strncpy(RUNTIME_CONFIG_BLOB.device_id, device_id, RUNTIME_DEVICE_ID_MAX_LEN);
        RUNTIME_CONFIG_BLOB.device_id[RUNTIME_DEVICE_ID_MAX_LEN] = '\0';

        len = sizeof(RUNTIME_CONFIG_BLOB.wifi_ssid);
        nvs_get_str(h, "wifi_ssid", RUNTIME_CONFIG_BLOB.wifi_ssid, &len);

        len = sizeof(RUNTIME_CONFIG_BLOB.wifi_password);
        nvs_get_str(h, "wifi_pass", RUNTIME_CONFIG_BLOB.wifi_password, &len);

        len = sizeof(RUNTIME_CONFIG_BLOB.mqtt_uri);
        nvs_get_str(h, "mqtt_uri", RUNTIME_CONFIG_BLOB.mqtt_uri, &len);

        len = sizeof(RUNTIME_CONFIG_BLOB.mqtt_username);
        nvs_get_str(h, "mqtt_user", RUNTIME_CONFIG_BLOB.mqtt_username, &len);

        len = sizeof(RUNTIME_CONFIG_BLOB.mqtt_password);
        nvs_get_str(h, "mqtt_pass", RUNTIME_CONFIG_BLOB.mqtt_password, &len);

        len = sizeof(RUNTIME_CONFIG_BLOB.template_id);
        nvs_get_str(h, "tmpl_id", RUNTIME_CONFIG_BLOB.template_id, &len);

        len = sizeof(RUNTIME_CONFIG_BLOB.relay_pins);
        nvs_get_blob(h, "relay_pins", RUNTIME_CONFIG_BLOB.relay_pins, &len);

        nvs_get_u8(h, "relay_alow", &RUNTIME_CONFIG_BLOB.relay_active_low_mask);

        // Ensure the blob signature is valid so existing callers work.
        strncpy(RUNTIME_CONFIG_BLOB.signature, PATCH_SIGNATURE_V2, PATCH_SIGNATURE_FIELD_SIZE - 1);
        RUNTIME_CONFIG_BLOB.signature[PATCH_SIGNATURE_FIELD_SIZE - 1] = '\0';

        ESP_LOGI(TAG, "Config loaded from NVS (device_id=%s)", device_id);
    }

    nvs_close(h);
    return found;
}

static void save_to_nvs(void)
{
    nvs_handle_t h;
    if (nvs_open(NVS_NAMESPACE, NVS_READWRITE, &h) != ESP_OK) {
        ESP_LOGW(TAG, "Failed to open NVS for write");
        return;
    }

    nvs_set_str(h, "device_id", RUNTIME_CONFIG_BLOB.device_id);
    nvs_set_str(h, "wifi_ssid", RUNTIME_CONFIG_BLOB.wifi_ssid);
    nvs_set_str(h, "wifi_pass", RUNTIME_CONFIG_BLOB.wifi_password);
    nvs_set_str(h, "mqtt_uri",  RUNTIME_CONFIG_BLOB.mqtt_uri);
    nvs_set_str(h, "mqtt_user", RUNTIME_CONFIG_BLOB.mqtt_username);
    nvs_set_str(h, "mqtt_pass", RUNTIME_CONFIG_BLOB.mqtt_password);
    nvs_set_str(h, "tmpl_id",   RUNTIME_CONFIG_BLOB.template_id);
    nvs_set_blob(h, "relay_pins", RUNTIME_CONFIG_BLOB.relay_pins, sizeof(RUNTIME_CONFIG_BLOB.relay_pins));
    nvs_set_u8(h, "relay_alow", RUNTIME_CONFIG_BLOB.relay_active_low_mask);

    nvs_commit(h);
    nvs_close(h);

    ESP_LOGI(TAG, "Config saved to NVS (device_id=%s)", RUNTIME_CONFIG_BLOB.device_id);
}

// ---------------------------------------------------------------------------

static void init_runtime_config(void)
{
    static bool inited = false;
    if (inited) return;

    // NVS is a dedicated flash partition that OTA never touches.
    // Always check it first so config survives firmware updates.
    if (load_from_nvs()) {
        inited = true;
        return;
    }

    // NVS empty. Check the embedded blob — it is patched in-place by the web
    // flasher before the binary is written to flash. A freshly USB-flashed
    // device will have non-empty fields here.
    bool blob_has_config = runtime_blob_valid_v2() &&
                           (RUNTIME_CONFIG_BLOB.device_id[0] != '\0' ||
                            RUNTIME_CONFIG_BLOB.wifi_ssid[0] != '\0');
    if (blob_has_config) {
        // Persist to NVS so the next OTA boot finds it there.
        save_to_nvs();
        inited = true;
        return;
    }

    // Nothing found — device needs provisioning via USB flasher.
    ESP_LOGW(TAG, "No config found in NVS or blob — device needs provisioning");
    inited = true;
}

const char *runtime_get_wifi_ssid(void)
{
    static char value[RUNTIME_WIFI_SSID_MAX_LEN + 1];
    static bool inited = false;
    if (!inited) {
        init_runtime_config();
        copy_runtime_or_default(value, sizeof(value),
                                RUNTIME_CONFIG_BLOB.wifi_ssid,
                                CONFIG_WIFI_SSID,
                                runtime_blob_valid_v2() || runtime_blob_valid_v1());
        inited = true;
    }
    return value;
}

const char *runtime_get_wifi_password(void)
{
    static char value[RUNTIME_WIFI_PASSWORD_MAX_LEN + 1];
    static bool inited = false;
    if (!inited) {
        init_runtime_config();
        copy_runtime_or_default(value, sizeof(value),
                                RUNTIME_CONFIG_BLOB.wifi_password,
                                CONFIG_WIFI_PASSWORD,
                                runtime_blob_valid_v2() || runtime_blob_valid_v1());
        inited = true;
    }
    return value;
}

const char *runtime_get_mqtt_broker_uri(void)
{
    static char value[RUNTIME_MQTT_URI_MAX_LEN + 1];
    static bool inited = false;
    if (!inited) {
        init_runtime_config();
        copy_runtime_or_default(value, sizeof(value),
                                RUNTIME_CONFIG_BLOB.mqtt_uri,
                                CONFIG_MQTT_BROKER_URI,
                                runtime_blob_valid_v2() || runtime_blob_valid_v1());
        inited = true;
    }
    return value;
}

const char *runtime_get_mqtt_username(void)
{
    static char value[RUNTIME_MQTT_USERNAME_MAX_LEN + 1];
    static bool inited = false;
    if (!inited) {
        init_runtime_config();
        copy_runtime_or_default(value, sizeof(value),
                                RUNTIME_CONFIG_BLOB.mqtt_username,
                                CONFIG_MQTT_USERNAME,
                                runtime_blob_valid_v2());
        inited = true;
    }
    return value;
}

const char *runtime_get_mqtt_password(void)
{
    static char value[RUNTIME_MQTT_PASSWORD_MAX_LEN + 1];
    static bool inited = false;
    if (!inited) {
        init_runtime_config();
        copy_runtime_or_default(value, sizeof(value),
                                RUNTIME_CONFIG_BLOB.mqtt_password,
                                CONFIG_MQTT_PASSWORD,
                                runtime_blob_valid_v2());
        inited = true;
    }
    return value;
}

const char *runtime_get_device_id(void)
{
    static char value[RUNTIME_DEVICE_ID_MAX_LEN + 1];
    static bool inited = false;
    if (!inited) {
        init_runtime_config();
        copy_runtime_or_default(value, sizeof(value),
                                RUNTIME_CONFIG_BLOB.device_id,
                                CONFIG_DEVICE_ID,
                                runtime_blob_valid_v2());
        inited = true;
    }
    return value;
}

const char *runtime_get_template_id(void)
{
    static char value[RUNTIME_TEMPLATE_ID_MAX_LEN + 1];
    static bool inited = false;
    if (!inited) {
        init_runtime_config();
        copy_runtime_or_default(value, sizeof(value),
                                RUNTIME_CONFIG_BLOB.template_id,
                                "switch",
                                runtime_blob_valid_v2());
        inited = true;
    }
    return value;
}


int runtime_get_relay_pin(uint8_t channel_index)
{
    if (!runtime_blob_valid_v2()) {
        return -1;
    }
    if (channel_index >= RUNTIME_RELAY_CHANNELS_MAX) {
        return -1;
    }
    uint8_t pin = RUNTIME_CONFIG_BLOB.relay_pins[channel_index];
    if (pin == 0) {
        return -1;
    }
    return (int)pin;
}

bool runtime_is_relay_active_low(uint8_t channel_index)
{
    if (!runtime_blob_valid_v2()) {
        return false;
    }
    if (channel_index >= RUNTIME_RELAY_CHANNELS_MAX) {
        return false;
    }
    uint8_t mask = (uint8_t)(1u << channel_index);
    return (RUNTIME_CONFIG_BLOB.relay_active_low_mask & mask) != 0;
}

bool runtime_config_has_overrides(void)
{
    init_runtime_config();
    if (runtime_blob_valid_v2()) {
        return (RUNTIME_CONFIG_BLOB.wifi_ssid[0] != '\0') ||
               (RUNTIME_CONFIG_BLOB.wifi_password[0] != '\0') ||
               (RUNTIME_CONFIG_BLOB.mqtt_uri[0] != '\0') ||
               (RUNTIME_CONFIG_BLOB.mqtt_username[0] != '\0') ||
               (RUNTIME_CONFIG_BLOB.mqtt_password[0] != '\0') ||
               (RUNTIME_CONFIG_BLOB.device_id[0] != '\0') ||
               (RUNTIME_CONFIG_BLOB.template_id[0] != '\0') ||
               (RUNTIME_CONFIG_BLOB.relay_pins[0] != 0) ||
               (RUNTIME_CONFIG_BLOB.relay_pins[1] != 0) ||
               (RUNTIME_CONFIG_BLOB.relay_pins[2] != 0) ||
               (RUNTIME_CONFIG_BLOB.relay_pins[3] != 0) ||
               (RUNTIME_CONFIG_BLOB.relay_active_low_mask != 0);
    }

    if (runtime_blob_valid_v1()) {
        runtime_config_blob_v1_t *v1 = (runtime_config_blob_v1_t *)&RUNTIME_CONFIG_BLOB;
        return (v1->wifi_ssid[0] != '\0') ||
               (v1->wifi_password[0] != '\0') ||
               (v1->mqtt_uri[0] != '\0');
    }

    return false;
}
