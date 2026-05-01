#pragma once

#include <stdbool.h>
#include <stdint.h>
#include "cJSON.h"

/**
 * @brief Common interface for all device drivers (sensors, actuators, etc.)
 */
typedef struct {
    /**
     * @brief Unique identifier for this template/driver (e.g., "switch")
     */
    const char *template_id;

    /**
     * @brief Initialize the hardware peripherals for this driver.
     */
    void (*init)(void);

    /**
     * @brief Handle an incoming MQTT control command.
     * @param action The parsed action string
     * @param params The parsed params JSON object
     * @param req_id The request ID (for sending acks)
     */
    void (*handle_mqtt)(const char *action, const cJSON *params, const char *req_id);

    /**
     * @brief Populate the current state into the provided JSON object.
     * @param state The JSON object to populate
     */
    void (*get_state)(cJSON *state);
} device_driver_t;

/**
 * @brief Find a device driver by its template ID.
 * @param template_id The template ID to search for
 * @return Pointer to the driver, or NULL if not found
 */
const device_driver_t *device_driver_find(const char *template_id);
