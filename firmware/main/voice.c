/*
 * voice.c — wake word detection (esp-sr/WakeNet9) → LiveKit voice session.
 *
 * State machine:
 *   IDLE        AFE tasks own I2S_NUM_0 (mic), listening for "Joy".
 *   WAKE_SENT   Wake word detected; MQTT wake published; waiting for voice
 *               credentials from the backend.
 *   ACTIVE      livekit_task owns both I2S buses; AFE paused.
 *
 * Audio resource hand-off:
 *   Idle  → Active : set s_afe_paused, wait 200 ms for feed task to drain,
 *                    then delete the I2S RX channel and open fresh channels
 *                    for the codec/LiveKit layer.
 *   Active → Idle  : tear down codec/LiveKit I2S, reinit mic RX, clear flag.
 */

#include <string.h>
#include <stdlib.h>

#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/event_groups.h"

#include "esp_log.h"
#include "esp_heap_caps.h"
#include "driver/i2s_std.h"
#include "mqtt_client.h"
#include "cJSON.h"

#include "model_path.h"
#include "esp_afe_sr_iface.h"
#include "esp_afe_sr_models.h"
#include "esp_afe_config.h"

#include "esp_codec_dev.h"
#include "esp_codec_dev_defaults.h"
#include "esp_capture_sink.h"
#include "esp_capture_defaults.h"
#include "av_render_default.h"
#include "esp_audio_enc_default.h"
#include "esp_audio_dec_default.h"
#include "livekit.h"

#include "voice.h"

static const char *TAG = "voice";

/* ── Pin assignments (match hardware; protected in switch driver) ─────────── */

#define I2S_MIC_SCK    GPIO_NUM_16
#define I2S_MIC_WS     GPIO_NUM_15
#define I2S_MIC_SD     GPIO_NUM_17
#define I2S_SPK_BCLK   GPIO_NUM_4
#define I2S_SPK_LRC    GPIO_NUM_5
#define I2S_SPK_DOUT   GPIO_NUM_6
#define SAMPLE_RATE    16000
/* INMP441: 24-bit audio left-justified in 32-bit I2S word — shift to 16-bit */
#define MIC_SHIFT      11

/* ── State machine ───────────────────────────────────────────────────────── */

typedef enum { VOICE_IDLE, VOICE_WAKE_SENT, VOICE_ACTIVE } voice_state_t;
static volatile voice_state_t s_state     = VOICE_IDLE;
static volatile bool          s_afe_paused = false;

/* ── Shared resources ────────────────────────────────────────────────────── */

static esp_mqtt_client_handle_t  s_mqtt       = NULL;
static char                      s_device_id[65];
static i2s_chan_handle_t          s_rx_chan    = NULL;
static const esp_afe_sr_iface_t *s_afe_handle = NULL;
static esp_afe_sr_data_t        *s_afe_data   = NULL;

/* LiveKit disconnect signal — created/destroyed per session */
static EventGroupHandle_t s_lk_eg = NULL;
#define LK_DONE_BIT BIT0

/* Session credentials passed (heap-allocated) to livekit_task */
typedef struct {
    char room_name[128];
    char url[128];
    char token[2048];
} livekit_session_t;

/* ── I2S mic helpers ─────────────────────────────────────────────────────── */

static void i2s_mic_init(void)
{
    i2s_chan_config_t cc = I2S_CHANNEL_DEFAULT_CONFIG(I2S_NUM_0, I2S_ROLE_MASTER);
    ESP_ERROR_CHECK(i2s_new_channel(&cc, NULL, &s_rx_chan));

    i2s_std_config_t sc = {
        .clk_cfg  = I2S_STD_CLK_DEFAULT_CONFIG(SAMPLE_RATE),
        .slot_cfg = I2S_STD_PHILIPS_SLOT_DEFAULT_CONFIG(
                        I2S_DATA_BIT_WIDTH_32BIT, I2S_SLOT_MODE_MONO),
        .gpio_cfg = {
            .mclk = I2S_GPIO_UNUSED,
            .bclk = I2S_MIC_SCK,
            .ws   = I2S_MIC_WS,
            .dout = I2S_GPIO_UNUSED,
            .din  = I2S_MIC_SD,
        },
    };
    sc.slot_cfg.slot_mask = I2S_STD_SLOT_LEFT;
    ESP_ERROR_CHECK(i2s_channel_init_std_mode(s_rx_chan, &sc));
    ESP_ERROR_CHECK(i2s_channel_enable(s_rx_chan));
}

static void i2s_mic_deinit(void)
{
    if (!s_rx_chan) return;
    i2s_channel_disable(s_rx_chan);
    i2s_del_channel(s_rx_chan);
    s_rx_chan = NULL;
}

/* ── AFE tasks ───────────────────────────────────────────────────────────── */

static void afe_feed_task(void *arg)
{
    int      n   = s_afe_handle->get_feed_chunksize(s_afe_data);
    int32_t *raw = heap_caps_malloc((size_t)n * sizeof(int32_t), MALLOC_CAP_SPIRAM);
    int16_t *pcm = heap_caps_malloc((size_t)n * sizeof(int16_t), MALLOC_CAP_INTERNAL);
    assert(raw && pcm);

#if CONFIG_VOICE_MIC_PEAK_LOG
    int log_tick = 0;
#endif

    ESP_LOGI(TAG, "afe_feed_task started: chunk=%d bytes=%d rx_chan=%p paused=%d",
             n, (int)(n * sizeof(int32_t)), (void *)s_rx_chan, (int)s_afe_paused);

    int err_log_count = 0;
    while (true) {
        if (s_afe_paused || !s_rx_chan) {
            vTaskDelay(pdMS_TO_TICKS(50));
            continue;
        }
        size_t got = 0;
        esp_err_t rc = i2s_channel_read(s_rx_chan, raw, (size_t)n * sizeof(int32_t),
                                        &got, pdMS_TO_TICKS(1000));
        if (rc != ESP_OK || got < (size_t)n * sizeof(int32_t)) {
            if (err_log_count < 5) {
                ESP_LOGE(TAG, "i2s_channel_read failed: rc=%s got=%d want=%d",
                         esp_err_to_name(rc), (int)got, (int)(n * sizeof(int32_t)));
                err_log_count++;
            }
            continue;
        }
        err_log_count = 0;
        for (int i = 0; i < n; i++) pcm[i] = (int16_t)(raw[i] >> MIC_SHIFT);
#if CONFIG_VOICE_MIC_PEAK_LOG
        {
            int16_t peak = 0;
            for (int i = 0; i < n; i++) {
                int16_t a = pcm[i] < 0 ? -pcm[i] : pcm[i];
                if (a > peak) peak = a;
            }
            if (++log_tick >= 62) { ESP_LOGI(TAG, "mic peak: %d", peak); log_tick = 0; }
        }
#endif
        s_afe_handle->feed(s_afe_data, pcm);
    }
}

static void afe_fetch_task(void *arg)
{
    while (true) {
        afe_fetch_result_t *r = s_afe_handle->fetch(s_afe_data);
        if (!r || r->ret_value == ESP_FAIL) continue;
        if (r->wakeup_state != WAKENET_DETECTED) continue;
        if (s_state != VOICE_IDLE) continue;  /* debounce — already handling a session */

        ESP_LOGI(TAG, "\"Hi Joy\" wake word detected — notifying backend (device=%s)", s_device_id);
        s_state = VOICE_WAKE_SENT;

        char topic[128];
        snprintf(topic, sizeof(topic), "sol/devices/%s/wake", s_device_id);
        int pub_id = esp_mqtt_client_publish(s_mqtt, topic, "{}", 2, 1, 0);
        if (pub_id >= 0) {
            ESP_LOGI(TAG, "wake signal published to backend (msg_id=%d)", pub_id);
        } else {
            ESP_LOGE(TAG, "failed to publish wake signal — MQTT error");
        }
    }
}

/* ── LiveKit room state callback ─────────────────────────────────────────── */

static void on_room_state(livekit_connection_state_t state, void *ctx)
{
    ESP_LOGI(TAG, "livekit state: %s", livekit_connection_state_str(state));
    if ((state == LIVEKIT_CONNECTION_STATE_DISCONNECTED ||
         state == LIVEKIT_CONNECTION_STATE_FAILED) && s_lk_eg) {
        xEventGroupSetBits(s_lk_eg, LK_DONE_BIT);
    }
}

/* ── LiveKit session task ────────────────────────────────────────────────── */

static void livekit_task(void *arg)
{
    livekit_session_t *sess = (livekit_session_t *)arg;
    livekit_room_handle_t room = NULL;
    i2s_chan_handle_t rx = NULL, tx = NULL;
    esp_capture_sink_handle_t capturer = NULL;
    av_render_handle_t renderer = NULL;

    /* Pause AFE and release the I2S RX channel */
    s_afe_paused = true;
    vTaskDelay(pdMS_TO_TICKS(200));  /* let afe_feed_task finish current read */
    i2s_mic_deinit();

    /* Open I2S_NUM_0 RX (mic) for codec layer */
    {
        i2s_chan_config_t cc = I2S_CHANNEL_DEFAULT_CONFIG(I2S_NUM_0, I2S_ROLE_MASTER);
        if (i2s_new_channel(&cc, NULL, &rx) != ESP_OK) goto cleanup;
        i2s_std_config_t sc = {
            .clk_cfg  = I2S_STD_CLK_DEFAULT_CONFIG(SAMPLE_RATE),
            .slot_cfg = I2S_STD_PHILIPS_SLOT_DEFAULT_CONFIG(
                            I2S_DATA_BIT_WIDTH_32BIT, I2S_SLOT_MODE_MONO),
            .gpio_cfg = { .mclk=I2S_GPIO_UNUSED, .bclk=I2S_MIC_SCK,
                          .ws=I2S_MIC_WS, .dout=I2S_GPIO_UNUSED, .din=I2S_MIC_SD },
        };
        sc.slot_cfg.slot_mask = I2S_STD_SLOT_LEFT;
        i2s_channel_init_std_mode(rx, &sc);
        i2s_channel_enable(rx);
    }

    /* Open I2S_NUM_1 TX (speaker) */
    {
        i2s_chan_config_t cc = I2S_CHANNEL_DEFAULT_CONFIG(I2S_NUM_1, I2S_ROLE_MASTER);
        cc.auto_clear = true;
        if (i2s_new_channel(&cc, &tx, NULL) != ESP_OK) goto cleanup;
        i2s_std_config_t sc = {
            .clk_cfg  = I2S_STD_CLK_DEFAULT_CONFIG(SAMPLE_RATE),
            .slot_cfg = I2S_STD_PHILIPS_SLOT_DEFAULT_CONFIG(
                            I2S_DATA_BIT_WIDTH_16BIT, I2S_SLOT_MODE_STEREO),
            .gpio_cfg = { .mclk=I2S_GPIO_UNUSED, .bclk=I2S_SPK_BCLK,
                          .ws=I2S_SPK_LRC, .dout=I2S_SPK_DOUT, .din=I2S_GPIO_UNUSED },
        };
        i2s_channel_init_std_mode(tx, &sc);
        i2s_channel_enable(tx);
    }

    /* Wrap I2S channels in codec devices */
    audio_codec_i2s_cfg_t mic_i2s = { .port=I2S_NUM_0, .rx_handle=rx, .tx_handle=NULL };
    const audio_codec_data_if_t *mic_if = audio_codec_new_i2s_data(&mic_i2s);
    esp_codec_dev_cfg_t rec_cfg = { .dev_type=ESP_CODEC_DEV_TYPE_IN, .codec_if=NULL, .data_if=mic_if };
    esp_codec_dev_handle_t rec = esp_codec_dev_new(&rec_cfg);

    audio_codec_i2s_cfg_t spk_i2s = { .port=I2S_NUM_1, .tx_handle=tx, .rx_handle=NULL };
    const audio_codec_data_if_t *spk_if = audio_codec_new_i2s_data(&spk_i2s);
    esp_codec_dev_cfg_t play_cfg = { .dev_type=ESP_CODEC_DEV_TYPE_OUT, .codec_if=NULL, .data_if=spk_if };
    esp_codec_dev_handle_t play = esp_codec_dev_new(&play_cfg);
    esp_codec_dev_set_out_vol(play, 70);

    /* esp_capture: mic → LiveKit (Opus, 16 kHz mono) */
    esp_capture_audio_dev_src_cfg_t src_cfg = { .record_handle = rec };
    esp_capture_audio_src_if_t *audio_src = esp_capture_new_audio_dev_src(&src_cfg);
    esp_capture_cfg_t cap_cfg = { .sync_mode=ESP_CAPTURE_SYNC_MODE_AUDIO, .audio_src=audio_src };
    if (esp_capture_open(&cap_cfg, &capturer) != ESP_OK || !capturer) goto cleanup;

    /* av_render: LiveKit → speaker */
    i2s_render_cfg_t rnd_i2s = { .play_handle = play };
    audio_render_handle_t audio_rnd = av_render_alloc_i2s_render(&rnd_i2s);
    av_render_cfg_t rnd_cfg = {
        .audio_render           = audio_rnd,
        .audio_raw_fifo_size    = 8 * 4096,
        .audio_render_fifo_size = 100 * 1024,
        .allow_drop_data        = false,
    };
    renderer = av_render_open(&rnd_cfg);
    if (!renderer) goto cleanup;
    av_render_audio_frame_info_t fi = { .sample_rate=SAMPLE_RATE, .channel=2, .bits_per_sample=16 };
    av_render_set_fixed_frame_info(renderer, &fi);

    /* Connect to LiveKit room */
    s_lk_eg = xEventGroupCreate();
    livekit_room_options_t opts = {
        .publish = {
            .kind = LIVEKIT_MEDIA_TYPE_AUDIO,
            .audio_encode = {
                .codec         = LIVEKIT_AUDIO_CODEC_OPUS,
                .sample_rate   = SAMPLE_RATE,
                .channel_count = 1,
            },
            .capturer = (esp_capture_handle_t)capturer,
        },
        .subscribe = {
            .kind     = LIVEKIT_MEDIA_TYPE_AUDIO,
            .renderer = renderer,
        },
        .on_state_changed = on_room_state,
    };

    if (livekit_room_create(&room, &opts) != ESP_OK ||
        livekit_room_connect(room, sess->url, sess->token) != ESP_OK) {
        ESP_LOGE(TAG, "livekit connect failed");
        xEventGroupSetBits(s_lk_eg, LK_DONE_BIT);
    } else {
        ESP_LOGI(TAG, "livekit connected: %s", sess->room_name);
        /* Wait for room disconnect from server or 10-min hard cap */
        xEventGroupWaitBits(s_lk_eg, LK_DONE_BIT, pdTRUE, pdTRUE,
                            pdMS_TO_TICKS(10 * 60 * 1000));
    }

cleanup:
    if (renderer) av_render_close(renderer);
    if (capturer) esp_capture_close(capturer);
    if (rx) { i2s_channel_disable(rx); i2s_del_channel(rx); }
    if (tx) { i2s_channel_disable(tx); i2s_del_channel(tx); }
    if (s_lk_eg) { vEventGroupDelete(s_lk_eg); s_lk_eg = NULL; }

    /* Restore mic for AFE and resume wake word detection */
    i2s_mic_init();
    s_afe_paused = false;
    s_state = VOICE_IDLE;

    free(sess);
    ESP_LOGI(TAG, "voice session ended — wake word detection resumed");
    vTaskDelete(NULL);
}

/* ── Public API ──────────────────────────────────────────────────────────── */

void voice_init(esp_mqtt_client_handle_t client, const char *device_id)
{
    s_mqtt = client;
    strncpy(s_device_id, device_id, sizeof(s_device_id) - 1);
    s_device_id[sizeof(s_device_id) - 1] = '\0';

    livekit_system_init();
    esp_audio_enc_register_default();
    esp_audio_dec_register_default();

    i2s_mic_init();

    srmodel_list_t *models = esp_srmodel_init("model");
    if (!models) {
        ESP_LOGE(TAG, "esp-sr models not found in 'model' partition — wake word disabled");
        return;
    }

    char *wn_model = esp_srmodel_filter(models, ESP_WN_PREFIX, NULL);
    if (!wn_model) {
        ESP_LOGE(TAG, "srmodels image has no wakenet model (wn9_*) — "
                 "firmware version was uploaded without a valid srmodels.bin; wake word disabled. "
                 "Delete sdkconfig, rebuild, and re-upload with build/srmodels/srmodels.bin as model.bin.");
        return;
    }
    ESP_LOGI(TAG, "wakenet model: %s", wn_model);

    afe_config_t *cfg = afe_config_init("M", models, AFE_TYPE_SR, AFE_MODE_LOW_COST);
    cfg->aec_init          = false;
    cfg->se_init           = false;
    cfg->wakenet_init      = true;
    cfg->memory_alloc_mode = AFE_MEMORY_ALLOC_MORE_PSRAM;

    s_afe_handle = esp_afe_handle_from_config(cfg);
    s_afe_data   = s_afe_handle->create_from_config(cfg);
    afe_config_free(cfg);

    xTaskCreatePinnedToCore(afe_feed_task,  "afe_feed",  8192, NULL, 5, NULL, 0);
    xTaskCreatePinnedToCore(afe_fetch_task, "afe_fetch", 8192, NULL, 5, NULL, 1);
    ESP_LOGI(TAG, "wake word detection started (device_id=%s)", s_device_id);
}

void voice_handle_session(const char *payload)
{
    if (s_state != VOICE_WAKE_SENT) {
        ESP_LOGW(TAG, "spurious voice session ignored (state=%d)", (int)s_state);
        return;
    }

    ESP_LOGI(TAG, "LiveKit token received from backend — parsing session credentials");

    cJSON *root = cJSON_Parse(payload);
    if (!root) {
        ESP_LOGW(TAG, "invalid voice session JSON");
        s_state = VOICE_IDLE;
        return;
    }

    const cJSON *jroom  = cJSON_GetObjectItemCaseSensitive(root, "room_name");
    const cJSON *jtoken = cJSON_GetObjectItemCaseSensitive(root, "token");
    const cJSON *jurl   = cJSON_GetObjectItemCaseSensitive(root, "url");

    if (!cJSON_IsString(jroom) || !cJSON_IsString(jtoken) || !cJSON_IsString(jurl)) {
        ESP_LOGW(TAG, "voice session JSON missing required fields");
        cJSON_Delete(root);
        s_state = VOICE_IDLE;
        return;
    }

    livekit_session_t *sess = calloc(1, sizeof(livekit_session_t));
    if (!sess) {
        cJSON_Delete(root);
        s_state = VOICE_IDLE;
        return;
    }

    strncpy(sess->room_name, jroom->valuestring,  sizeof(sess->room_name)  - 1);
    strncpy(sess->token,     jtoken->valuestring, sizeof(sess->token)      - 1);
    strncpy(sess->url,       jurl->valuestring,   sizeof(sess->url)        - 1);
    cJSON_Delete(root);

    ESP_LOGI(TAG, "LiveKit token retrieved: room=%s url=%s", sess->room_name, sess->url);

    s_state = VOICE_ACTIVE;
    xTaskCreatePinnedToCore(livekit_task, "livekit", 32768, sess, 5, NULL, 1);
}
