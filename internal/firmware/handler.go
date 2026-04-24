package firmware

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/muaathrifath/sol-core/internal/user"
)

type Handler struct {
	store    *Store
	versions *VersionRepository
	builds   *BuildRepository
	builder  *Builder
}

func NewHandler(store *Store, versions *VersionRepository, builds *BuildRepository, builder *Builder) *Handler {
	return &Handler{
		store:    store,
		versions: versions,
		builds:   builds,
		builder:  builder,
	}
}

func (h *Handler) Build(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TemplateID  string `json:"template_id"`
		TargetBoard string `json:"target_board"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if body.TemplateID == "" || body.TargetBoard == "" {
		http.Error(w, `{"error":"template_id and target_board are required"}`, http.StatusBadRequest)
		return
	}

	jobID, err := h.builder.Submit(r.Context(), body.TemplateID, body.TargetBoard)
	if err != nil {
		slog.Error("submit build", "error", err)
		http.Error(w, `{"error":"failed to start build"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": jobID})
}

func (h *Handler) GetBuild(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	build, err := h.builds.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(build)
}

func (h *Handler) UpdateBuildStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Status            BuildStatus `json:"status"`
		FirmwareVersionID string      `json:"firmware_version_id"`
		Logs              string      `json:"logs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	var err error
	if body.Status == StatusSuccess && body.FirmwareVersionID != "" {
		err = h.builds.UpdateSuccess(r.Context(), id, body.FirmwareVersionID, body.Logs)
	} else {
		err = h.builds.UpdateStatus(r.Context(), id, body.Status, body.Logs)
	}

	if err != nil {
		slog.Error("update build status", "error", err)
		http.Error(w, `{"error":"failed to update"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AppendBuildLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Logs string `json:"logs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if err := h.builds.AppendLogs(r.Context(), id, body.Logs); err != nil {
		slog.Error("append build logs", "error", err)
		http.Error(w, `{"error":"failed to append"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListTargets(w http.ResponseWriter, r *http.Request) {
	targets := []string{"esp32", "esp32s2", "esp32s3", "esp32c3", "esp32c6", "esp32h2"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(targets)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	templateID := strings.TrimSpace(r.URL.Query().Get("template_id"))
	versions, err := h.versions.List(r.Context(), templateID)
	if err != nil {
		slog.Error("list firmware", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(versions)
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	u := user.FromContext(r.Context())
	if u == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max
		http.Error(w, `{"error":"file too large or invalid form"}`, http.StatusBadRequest)
		return
	}
	templateID := strings.TrimSpace(r.FormValue("template_id"))
	if templateID == "" {
		http.Error(w, `{"error":"template_id is required"}`, http.StatusBadRequest)
		return
	}

	version := strings.TrimSpace(r.FormValue("version"))
	if version == "" {
		version = time.Now().UTC().Format("2006-01-02T15-04-05Z")
	}

	bootloader, bootloaderHeader, err := r.FormFile("bootloader")
	if err != nil {
		http.Error(w, `{"error":"missing bootloader file"}`, http.StatusBadRequest)
		return
	}
	defer bootloader.Close()

	partition, partitionHeader, err := r.FormFile("partition_table")
	if err != nil {
		http.Error(w, `{"error":"missing partition_table file"}`, http.StatusBadRequest)
		return
	}
	defer partition.Close()

	app, appHeader, err := r.FormFile("app")
	if err != nil {
		http.Error(w, `{"error":"missing app file"}`, http.StatusBadRequest)
		return
	}
	defer app.Close()

	bootloaderKey, err := h.store.UploadVersioned(r.Context(), templateID, version, "bootloader.bin", bootloader, bootloaderHeader.Size)
	if err != nil {
		slog.Error("upload bootloader", "error", err)
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}

	partitionKey, err := h.store.UploadVersioned(r.Context(), templateID, version, "partition_table.bin", partition, partitionHeader.Size)
	if err != nil {
		slog.Error("upload partition", "error", err)
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}

	appKey, err := h.store.UploadVersioned(r.Context(), templateID, version, "app.bin", app, appHeader.Size)
	if err != nil {
		slog.Error("upload app", "error", err)
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}

	var sourceKey *string
	source, sourceHeader, err := r.FormFile("source")
	if err == nil {
		defer source.Close()
		key, uploadErr := h.store.UploadVersioned(r.Context(), templateID, version, "source.bin", source, sourceHeader.Size)
		if uploadErr != nil {
			slog.Error("upload source", "error", uploadErr)
			http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
			return
		}
		sourceKey = &key
	}

	sizeBytes := appHeader.Size
	v := FirmwareVersion{
		ID:            uuid.NewString(),
		TemplateID:    templateID,
		Version:       version,
		BootloaderKey: bootloaderKey,
		PartitionKey:  partitionKey,
		AppKey:        appKey,
		SourceKey:     sourceKey,
		SizeBytes:     &sizeBytes,
		UploadedBy:    &u.ID,
		CreatedAt:     time.Now(),
	}

	if err := h.versions.Create(r.Context(), &v); err != nil {
		slog.Error("insert firmware version", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(v)
}

func (h *Handler) Download(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	reader, err := h.store.Download(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+id)
	io.Copy(w, reader)
}

func (h *Handler) PresignedURL(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := h.versions.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	url, err := h.store.PresignedURL(r.Context(), v.AppKey, 24*time.Hour)
	if err != nil {
		slog.Error("presign firmware", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}

func (h *Handler) DownloadByVersionID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v, err := h.versions.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	reader, err := h.store.Download(r.Context(), v.AppKey)
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s-%s-app.bin", v.TemplateID, v.Version))
	io.Copy(w, reader)
}
