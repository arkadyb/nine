package httpapi

import (
	"net/http"
	"time"

	"nine/internal/app/model"
	"nine/internal/app/store"
)

type ManifestStore interface {
	UpsertSegment(assetID string, seg store.Segment)
	GetManifest(assetID string) (store.Manifest, bool)
}

type Handler struct {
	store     ManifestStore
	startedAt time.Time
	mux       *http.ServeMux
}

func NewHandler(store ManifestStore, startedAt time.Time) *Handler {
	h := &Handler{
		store:     store,
		startedAt: startedAt,
		mux:       http.NewServeMux(),
	}
	h.mux.HandleFunc("POST /ingest", h.ingest)
	h.mux.HandleFunc("GET /manifest/{asset_id...}", h.manifest)
	h.mux.HandleFunc("GET /health", h.health)
	h.mux.HandleFunc("/", h.notFound)
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) ingest(w http.ResponseWriter, r *http.Request) {
	req, err := model.ReadIngestRequest(r.Body)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Could not decode request: JSON parsing failed")
		return
	}
	if field := model.MissingIngestField(req); field != "" {
		writeJSONError(w, http.StatusBadRequest, "missing required field: "+field)
		return
	}

	assetID := *req.AssetID
	segment := store.Segment{
		Index:      *req.SegmentIndex,
		CDNURL:     *req.CDNURL,
		ReceivedAt: *req.ReceivedAt,
	}
	h.store.UpsertSegment(assetID, segment)

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (h *Handler) manifest(w http.ResponseWriter, r *http.Request) {
	assetID := r.PathValue("asset_id")
	if assetID == "" {
		writeJSONError(w, http.StatusNotFound, "asset not found")
		return
	}

	manifest, ok := h.store.GetManifest(assetID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "asset not found")
		return
	}

	resp := model.ManifestResponse{
		AssetID:     assetID,
		Segments:    make([]model.ManifestSegment, 0, len(manifest.Segments)),
		LastUpdated: manifest.LastUpdated,
	}
	for _, seg := range manifest.Segments {
		resp.Segments = append(resp.Segments, model.ManifestSegment{
			Index:  seg.Index,
			CDNURL: seg.CDNURL,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	uptime := int(time.Since(h.startedAt).Seconds())
	if uptime < 0 {
		uptime = 0
	}
	writeJSON(w, http.StatusOK, model.HealthResponse{
		Status:        "ok",
		UptimeSeconds: uptime,
	})
}

func (h *Handler) notFound(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
