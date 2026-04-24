#include "certs.h"
#include <string.h>
#include "esp_partition.h"
#include "esp_log.h"
#include "esp_heap_caps.h"

static const char *TAG = "certs";

#define SLOT_SIZE 0x1000 // 4KB

static char *read_slot(const esp_partition_t *partition, uint32_t offset)
{
    char *buf = heap_caps_malloc(SLOT_SIZE, MALLOC_CAP_8BIT);
    if (!buf) {
        return NULL;
    }

    esp_err_t err = esp_partition_read(partition, offset, buf, SLOT_SIZE);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "Failed to read partition at offset 0x%lx: %s", offset, esp_err_to_name(err));
        free(buf);
        return NULL;
    }

    // Ensure null termination. We check if the data starts with standard PEM headers.
    // If the partition is empty (all 0xFF), we return NULL.
    if ((uint8_t)buf[0] == 0xFF) {
        free(buf);
        return NULL;
    }

    // The partition might not be fully filled, ensure it's null terminated within SLOT_SIZE
    bool found_null = false;
    for (int i = 0; i < SLOT_SIZE; i++) {
        if (buf[i] == '\0') {
            found_null = true;
            break;
        }
    }

    if (!found_null) {
        // If no null terminator, force it at the end
        buf[SLOT_SIZE - 1] = '\0';
    }

    return buf;
}

esp_err_t certs_load(cert_bundle_t *bundle)
{
    const esp_partition_t *partition = esp_partition_find_first(0x99, 0x99, CERT_PARTITION_LABEL);
    if (!partition) {
        ESP_LOGW(TAG, "Certificates partition '%s' not found", CERT_PARTITION_LABEL);
        return ESP_ERR_NOT_FOUND;
    }

    bundle->ca_cert = read_slot(partition, CA_CERT_OFFSET);
    bundle->client_cert = read_slot(partition, CLIENT_CERT_OFFSET);
    bundle->client_key = read_slot(partition, CLIENT_KEY_OFFSET);

    if (!bundle->ca_cert || !bundle->client_cert || !bundle->client_key) {
        certs_free(bundle);
        return ESP_FAIL;
    }

    return ESP_OK;
}

void certs_free(cert_bundle_t *bundle)
{
    if (bundle->ca_cert) {
        free(bundle->ca_cert);
        bundle->ca_cert = NULL;
    }
    if (bundle->client_cert) {
        free(bundle->client_cert);
        bundle->client_cert = NULL;
    }
    if (bundle->client_key) {
        free(bundle->client_key);
        bundle->client_key = NULL;
    }
}
