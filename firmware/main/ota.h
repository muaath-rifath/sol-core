#pragma once

#include "esp_err.h"

/**
 * Start OTA update from the given firmware URL.
 * Downloads the binary, writes to inactive OTA partition, and reboots.
 * This function does not return on success (device reboots).
 */
esp_err_t ota_start(const char *url);

/**
 * Call on boot to validate the running OTA image.
 * If booted from a new OTA image, marks it valid to prevent rollback.
 */
void ota_validate_image(void);
