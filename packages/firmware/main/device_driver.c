#include "device_driver.h"
#include <string.h>
#include <stdio.h>

// External declarations for available drivers
// To add a new driver, add its extern declaration here and include it in the s_drivers array.
extern const device_driver_t s_switch_driver;

static const device_driver_t *s_drivers[] = {
    &s_switch_driver,
    NULL // Terminator
};

const device_driver_t *device_driver_find(const char *template_id) {
    if (!template_id || template_id[0] == '\0') {
        return s_drivers[0];
    }

    for (int i = 0; s_drivers[i] != NULL; i++) {
        if (strcmp(s_drivers[i]->template_id, template_id) == 0) {
            return s_drivers[i];
        }
    }

    // Fallback to switch driver if template not found
    return s_drivers[0];
}
