package firmware

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

type Handler struct {
	store *Store
}

func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	objects, err := h.store.List(r.Context())
	if err != nil {
		slog.Error("list firmware", "error", err)
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	type item struct {
		Name         string `json:"name"`
		Size         int64  `json:"size"`
		LastModified string `json:"last_modified"`
	}
	var items []item
	for _, obj := range objects {
		items = append(items, item{
			Name:         obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified.String(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(100 << 20); err != nil { // 100MB max
		http.Error(w, `{"error":"file too large or invalid form"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("firmware")
	if err != nil {
		http.Error(w, `{"error":"missing firmware file"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	if err := h.store.Upload(r.Context(), header.Filename, file, header.Size, "application/octet-stream"); err != nil {
		slog.Error("upload firmware", "error", err)
		http.Error(w, `{"error":"upload failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"name": header.Filename})
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
