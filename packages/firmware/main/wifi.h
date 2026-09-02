#pragma once

#include "esp_err.h"

/**
 * Initialize WiFi in station mode and connect.
 * Blocks until connected or max retries exceeded.
 * Returns ESP_OK on success, ESP_FAIL on failure.
 */
esp_err_t wifi_station_init(const char *ssid, const char *password);

/**
 * Get the current IPv4 address string safely.
 * Returns "0.0.0.0" if not connected.
 */
const char* wifi_get_ip(void);
