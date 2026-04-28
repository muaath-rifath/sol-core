#pragma once

#include "esp_err.h"
#include <stdbool.h>

typedef void (*ota_progress_cb_t)(const char *status, int progress,
                                  const char *message, const char *error);

typedef bool (*ota_cancel_cb_t)(void);

/**
 * Start OTA update from the given firmware URL.
 * Downloads the binary, writes to inactive OTA partition, and reboots.
 * This function does not return on success (device reboots).
 */
esp_err_t ota_start(const char *url, ota_progress_cb_t progress_cb,
                    ota_cancel_cb_t cancel_cb);

/**
 * Call on boot to validate the running OTA image.
 * If booted from a new OTA image, marks it valid to prevent rollback.
 */
void ota_validate_image(void);
