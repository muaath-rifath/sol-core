#include "ota.h"

#include "esp_log.h"
#include "esp_ota_ops.h"
#include "esp_http_client.h"
#include "esp_partition.h"

#define OTA_BUF_SIZE 4096

static const char *TAG = "ota";

void ota_validate_image(void)
{
    const esp_partition_t *running = esp_ota_get_running_partition();
    esp_ota_img_states_t ota_state;

    if (esp_ota_get_state_partition(running, &ota_state) == ESP_OK) {
        if (ota_state == ESP_OTA_IMG_PENDING_VERIFY) {
            ESP_LOGI(TAG, "OTA image pending verification, marking valid");
            esp_ota_mark_app_valid_cancel_rollback();
        }
    }
}

esp_err_t ota_start(const char *url)
{
    ESP_LOGI(TAG, "Starting OTA from: %s", url);

    esp_http_client_config_t http_config = {
        .url = url,
        .timeout_ms = 30000,
    };

    esp_http_client_handle_t client = esp_http_client_init(&http_config);
    if (!client) {
        ESP_LOGE(TAG, "Failed to init HTTP client");
        return ESP_FAIL;
    }

    esp_err_t err = esp_http_client_open(client, 0);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "Failed to open HTTP connection: %s", esp_err_to_name(err));
        esp_http_client_cleanup(client);
        return err;
    }

    int content_length = esp_http_client_fetch_headers(client);
    ESP_LOGI(TAG, "Firmware size: %d bytes", content_length);

    const esp_partition_t *update_partition = esp_ota_get_next_update_partition(NULL);
    if (!update_partition) {
        ESP_LOGE(TAG, "No OTA partition available");
        esp_http_client_cleanup(client);
        return ESP_FAIL;
    }

    ESP_LOGI(TAG, "Writing to partition: %s (offset 0x%lx)",
             update_partition->label, update_partition->address);

    esp_ota_handle_t ota_handle;
    err = esp_ota_begin(update_partition, OTA_SIZE_UNKNOWN, &ota_handle);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "esp_ota_begin failed: %s", esp_err_to_name(err));
        esp_http_client_cleanup(client);
        return err;
    }

    char *buf = malloc(OTA_BUF_SIZE);
    if (!buf) {
        ESP_LOGE(TAG, "Failed to allocate OTA buffer");
        esp_ota_abort(ota_handle);
        esp_http_client_cleanup(client);
        return ESP_ERR_NO_MEM;
    }

    int total_read = 0;
    int data_read;
    while ((data_read = esp_http_client_read(client, buf, OTA_BUF_SIZE)) > 0) {
        err = esp_ota_write(ota_handle, buf, data_read);
        if (err != ESP_OK) {
            ESP_LOGE(TAG, "esp_ota_write failed: %s", esp_err_to_name(err));
            break;
        }
        total_read += data_read;
    }

    free(buf);
    esp_http_client_cleanup(client);

    if (err != ESP_OK || data_read < 0) {
        ESP_LOGE(TAG, "OTA download failed");
        esp_ota_abort(ota_handle);
        return ESP_FAIL;
    }

    ESP_LOGI(TAG, "Downloaded %d bytes", total_read);

    err = esp_ota_end(ota_handle);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "esp_ota_end failed: %s", esp_err_to_name(err));
        return err;
    }

    err = esp_ota_set_boot_partition(update_partition);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "esp_ota_set_boot_partition failed: %s", esp_err_to_name(err));
        return err;
    }

    ESP_LOGI(TAG, "OTA successful, rebooting...");
    esp_restart();

    return ESP_OK; // unreachable
}
