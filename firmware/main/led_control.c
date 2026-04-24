#include "led_control.h"

#include "led_strip.h"
#include "esp_log.h"

#define LED_GPIO  48
#define LED_COUNT 1

static const char *TAG = "led";
static led_strip_handle_t led_strip;

void led_init(void)
{
    led_strip_config_t strip_config = {
        .strip_gpio_num = LED_GPIO,
        .max_leds = LED_COUNT,
        .led_model = LED_MODEL_WS2812,
        .flags.invert_out = false,
    };

    led_strip_rmt_config_t rmt_config = {
        .clk_src = RMT_CLK_SRC_DEFAULT,
        .resolution_hz = 10 * 1000 * 1000,
        .flags.with_dma = false,
    };

    esp_err_t ret = led_strip_new_rmt_device(&strip_config, &rmt_config, &led_strip);
    if (ret != ESP_OK) {
        ESP_LOGE(TAG, "Failed to init LED strip: %s", esp_err_to_name(ret));
        return;
    }

    led_strip_clear(led_strip);
    ESP_LOGI(TAG, "LED initialized on GPIO %d", LED_GPIO);
}

void led_set_color(uint8_t r, uint8_t g, uint8_t b)
{
    if (!led_strip) return;
    led_strip_set_pixel(led_strip, 0, r, g, b);
    led_strip_refresh(led_strip);
    ESP_LOGI(TAG, "Color set: R=%d G=%d B=%d", r, g, b);
}

void led_off(void)
{
    if (!led_strip) return;
    led_strip_clear(led_strip);
    ESP_LOGI(TAG, "LED off");
}
