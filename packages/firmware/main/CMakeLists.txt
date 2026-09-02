file(GLOB_RECURSE DRIVER_SRCS "drivers/*.c")

idf_component_register(SRCS "runtime_config.c" "main.c" "wifi.c" "ota.c" "device_driver.c" ${DRIVER_SRCS} "certs.c" "voice.c"
                       INCLUDE_DIRS "."
                       REQUIRES esp_wifi nvs_flash esp_http_client esp_https_ota app_update mqtt json esp_driver_gpio mbedtls
                                 esp_driver_i2s espressif__esp-sr livekit__livekit
                                 espressif__esp_codec_dev esp_lcd)
