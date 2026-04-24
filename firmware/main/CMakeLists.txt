idf_component_register(SRCS "runtime_config.c" "main.c" "wifi.c" "led_control.c" "ota.c" "dht_sensor.c" "smart_plug.c"
                       INCLUDE_DIRS "."
                       REQUIRES led_strip esp_wifi nvs_flash esp_http_client esp_https_ota app_update mqtt json esp_driver_gpio)
