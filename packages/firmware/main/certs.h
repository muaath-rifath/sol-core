#pragma once

#include <stddef.h>
#include <stdint.h>
#include "esp_err.h"

#define CERT_PARTITION_LABEL "certs"

// Offset in the certs partition
// Total partition size is 0x4000 (16KB)
#define CA_CERT_OFFSET      0x0000    // 4KB slot
#define CLIENT_CERT_OFFSET  0x1000    // 4KB slot
#define CLIENT_KEY_OFFSET   0x2000    // 4KB slot

typedef struct {
    char *ca_cert;
    char *client_cert;
    char *client_key;
} cert_bundle_t;

/**
 * @brief Load certificates from the dedicated flash partition.
 * Memory for pointers is allocated on the heap and must be freed by the caller.
 */
esp_err_t certs_load(cert_bundle_t *bundle);

/**
 * @brief Free memory allocated by certs_load.
 */
void certs_free(cert_bundle_t *bundle);
