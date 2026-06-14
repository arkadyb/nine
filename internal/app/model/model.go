package model

import (
	"encoding/json"
	"errors"
	"io"
	"time"
)

type IngestRequest struct {
	AssetID      *string    `json:"asset_id"`
	SegmentIndex *int       `json:"segment_index"`
	CDNURL       *string    `json:"cdn_url"`
	ReceivedAt   *time.Time `json:"received_at"`
}

type ManifestResponse struct {
	AssetID     string            `json:"asset_id"`
	Segments    []ManifestSegment `json:"segments"`
	LastUpdated time.Time         `json:"last_updated"`
}

type ManifestSegment struct {
	Index  int    `json:"index"`
	CDNURL string `json:"cdn_url"`
}

type HealthResponse struct {
	Status        string `json:"status"`
	UptimeSeconds int    `json:"uptime_seconds"`
}

func ReadIngestRequest(r io.Reader) (IngestRequest, error) {
	var req IngestRequest
	dec := json.NewDecoder(r)
	if err := dec.Decode(&req); err != nil {
		return IngestRequest{}, err
	}
	if err := dec.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		return IngestRequest{}, err
	}
	return req, nil
}

func MissingIngestField(req IngestRequest) string {
	switch {
	case req.AssetID == nil || *req.AssetID == "":
		return "asset_id"
	case req.SegmentIndex == nil:
		return "segment_index"
	case req.CDNURL == nil || *req.CDNURL == "":
		return "cdn_url"
	case req.ReceivedAt == nil:
		return "received_at"
	default:
		return ""
	}
}
