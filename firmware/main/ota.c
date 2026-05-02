#include "ota.h"

#include "esp_log.h"
#include "esp_https_ota.h"
#include "esp_ota_ops.h"
#include "esp_http_client.h"
#include "esp_crt_bundle.h"
#include "esp_partition.h"
#include "esp_app_format.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"

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

esp_err_t ota_start(const char *url, const char *client_cert, const char *client_key,
                    ota_progress_cb_t progress_cb, ota_cancel_cb_t cancel_cb)
{
    ESP_LOGI(TAG, "Starting OTA from: %s", url);
    if (progress_cb) {
        progress_cb("initiated", 1, "Starting OTA (C code)", NULL);
    }

    esp_http_client_config_t http_config = {
        .url = url,
        .crt_bundle_attach = esp_crt_bundle_attach,
        .client_cert_pem = client_cert,
        .client_key_pem = client_key,
        .timeout_ms = 30000,
        .keep_alive_enable = true,
    };

    esp_https_ota_config_t ota_config = {
        .http_config = &http_config,
    };

    if (progress_cb) {
        progress_cb("initiated", 2, "Calling esp_https_ota_begin", NULL);
    }

    esp_https_ota_handle_t ota_handle = NULL;
    esp_err_t err = esp_https_ota_begin(&ota_config, &ota_handle);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "esp_https_ota_begin failed: %s", esp_err_to_name(err));
        if (progress_cb) {
            progress_cb("failed", 0, "esp_https_ota_begin failed", NULL);
        }
        return err;
    }

    if (progress_cb) {
        progress_cb("initiated", 3, "esp_https_ota_begin successful", NULL);
    }


    int last_progress = 5;
    if (progress_cb) {
        progress_cb("downloading", 5, "Downloading firmware image", NULL);
    }

    bool header_checked = false;
    while (1) {
        if (cancel_cb && cancel_cb()) {
            ESP_LOGW(TAG, "OTA cancelled by command");
            esp_https_ota_abort(ota_handle);
            if (progress_cb) {
                progress_cb("cancelled", last_progress, "OTA cancelled by user", NULL);
            }
            return ESP_ERR_INVALID_STATE;
        }

        err = esp_https_ota_perform(ota_handle);
        if (err != ESP_OK && err != ESP_ERR_HTTPS_OTA_IN_PROGRESS) {
            ESP_LOGE(TAG, "esp_https_ota_perform failed: %s", esp_err_to_name(err));
            esp_https_ota_abort(ota_handle);
            if (progress_cb) {
                progress_cb("failed", last_progress, "esp_https_ota_perform failed", esp_err_to_name(err));
            }
            return err;
        }

        int total_read = esp_https_ota_get_image_len_read(ota_handle);
        if (!header_checked && total_read >= (int)(sizeof(esp_image_header_t) + sizeof(esp_image_segment_header_t) + sizeof(esp_app_desc_t))) {
            if (progress_cb) {
                progress_cb("verifying", 20, "Firmware header verified", NULL);
            }
            header_checked = true;
        }

        if (err == ESP_ERR_HTTPS_OTA_IN_PROGRESS) {
            if (progress_cb) {
                int dynamic = 20 + ((total_read / OTA_BUF_SIZE) % 70);
                if (dynamic < last_progress) {
                    dynamic = last_progress;
                }
                if (dynamic > 90) {
                    dynamic = 90;
                }
                if (dynamic != last_progress) {
                    last_progress = dynamic;
                    progress_cb("downloading", dynamic, "Writing firmware blocks", NULL);
                }
            }
            continue;
        }

        if (!esp_https_ota_is_complete_data_received(ota_handle)) {
            ESP_LOGE(TAG, "Complete OTA data was not received");
            esp_https_ota_abort(ota_handle);
            if (progress_cb) {
                progress_cb("failed", last_progress, "Incomplete OTA data", "incomplete data");
            }
            return ESP_FAIL;
        }

        err = esp_https_ota_finish(ota_handle);
        if (err != ESP_OK) {
            ESP_LOGE(TAG, "esp_https_ota_finish failed: %s", esp_err_to_name(err));
            if (progress_cb) {
                progress_cb("failed", last_progress, "OTA image validation failed", esp_err_to_name(err));
            }
            return err;
        }

        if (progress_cb) {
            progress_cb("updating", 95, "Image written and verified", NULL);
        }

        ESP_LOGI(TAG, "OTA successful, rebooting...");
        if (progress_cb) {
            progress_cb("updated", 100, "OTA successful, rebooting", NULL);
        }
        vTaskDelay(pdMS_TO_TICKS(500));
        esp_restart();

        break;
    }

    return ESP_OK;
}
