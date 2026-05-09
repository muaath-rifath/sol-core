-- Remap GPIO 15 (I2S MIC WS / INMP441) → GPIO 21 for any relay channel assigned to it.
UPDATE appliances SET gpio_pin = 21 WHERE gpio_pin = 15;

-- NULL out any relay channels still assigned to I2S audio pins that have no safe remap.
-- Pins 4=SPK_BCLK, 5=SPK_LRC, 6=SPK_DOUT (MAX98357A), 16=MIC_SCK, 17=MIC_SD (INMP441).
UPDATE appliances SET gpio_pin = NULL WHERE gpio_pin IN (4, 5, 6, 16, 17);
