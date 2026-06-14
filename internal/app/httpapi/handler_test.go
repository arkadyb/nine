package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"nine/internal/app/httpapi"
	"nine/internal/app/model"
	"nine/internal/app/store"
)

func TestIngestAndManifestOrdered(t *testing.T) {
	st := store.New()
	handler := httpapi.NewHandler(st, time.Unix(0, 0).UTC())

	for _, payload := range []string{
		`{"asset_id":"show/scooby/season/1/ep4","segment_index":5,"cdn_url":"https://cdn.example.com/scooby/s1e4/seg5.ts","received_at":"2024-05-01T10:00:00Z"}`,
		`{"asset_id":"show/scooby/season/1/ep4","segment_index":2,"cdn_url":"https://cdn.example.com/scooby/s1e4/seg2.ts","received_at":"2024-05-01T10:00:01Z"}`,
		`{"asset_id":"show/scooby/season/1/ep4","segment_index":0,"cdn_url":"https://cdn.example.com/scooby/s1e4/seg0.ts","received_at":"2024-05-01T10:00:02Z"}`,
		`{"asset_id":"show/scooby/season/1/ep4","segment_index":3,"cdn_url":"https://cdn.example.com/scooby/s1e4/seg3.ts","received_at":"2024-05-01T10:00:03Z"}`,
		`{"asset_id":"show/scooby/season/1/ep4","segment_index":1,"cdn_url":"https://cdn.example.com/scooby/s1e4/seg1.ts","received_at":"2024-05-01T10:00:04Z"}`,
		`{"asset_id":"show/scooby/season/1/ep4","segment_index":4,"cdn_url":"https://cdn.example.com/scooby/s1e4/seg4.ts","received_at":"2024-05-01T10:00:05Z"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/manifest/show/scooby/season/1/ep4", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp model.ManifestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if got, want := len(resp.Segments), 6; got != want {
		t.Fatalf("expected %d segments, got %d", want, got)
	}
	for i, seg := range resp.Segments {
		if seg.Index != i {
			t.Fatalf("expected segment %d at position %d, got %d", i, i, seg.Index)
		}
	}
}

func TestDuplicateIngestDoesNotDuplicateManifest(t *testing.T) {
	st := store.New()
	handler := httpapi.NewHandler(st, time.Unix(0, 0).UTC())

	payloads := []string{
		`{"asset_id":"show/thetaste/season/1/ep2","segment_index":0,"cdn_url":"https://cdn.example.com/thetaste/s1e2/seg0.ts","received_at":"2024-05-01T10:00:00Z"}`,
		`{"asset_id":"show/thetaste/season/1/ep2","segment_index":1,"cdn_url":"https://cdn.example.com/thetaste/s1e2/seg1.ts","received_at":"2024-05-01T10:00:01Z"}`,
		`{"asset_id":"show/thetaste/season/1/ep2","segment_index":0,"cdn_url":"https://cdn.example.com/thetaste/s1e2/seg0.ts","received_at":"2024-05-01T10:00:02Z"}`,
		`{"asset_id":"show/thetaste/season/1/ep2","segment_index":1,"cdn_url":"https://cdn.example.com/thetaste/s1e2/seg1.ts","received_at":"2024-05-01T10:00:03Z"}`,
		`{"asset_id":"show/thetaste/season/1/ep2","segment_index":2,"cdn_url":"https://cdn.example.com/thetaste/s1e2/seg2.ts","received_at":"2024-05-01T10:00:04Z"}`,
	}
	for _, payload := range payloads {
		req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(payload))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/manifest/show/thetaste/season/1/ep2", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp model.ManifestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	if got, want := len(resp.Segments), 3; got != want {
		t.Fatalf("expected %d unique segments, got %d", want, got)
	}
}

func TestIngestValidationErrors(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		status int
		errMsg string
	}{
		{
			name:   "invalid json",
			body:   `{ "asset_id": "show/thunderbirds", "segment_index": ]]]`,
			status: http.StatusBadRequest,
			errMsg: "Could not decode request: JSON parsing failed",
		},
		{
			name:   "wrong type",
			body:   `{"asset_id":"show/thunderbirds/season/1/ep1","segment_index":"four","cdn_url":"https://cdn.example.com/thunderbirds/s1e1/seg4.ts","received_at":"2024-05-01T10:00:00Z"}`,
			status: http.StatusBadRequest,
			errMsg: "Could not decode request: JSON parsing failed",
		},
		{
			name:   "empty body",
			body:   "",
			status: http.StatusBadRequest,
			errMsg: "Could not decode request: JSON parsing failed",
		},
		{
			name:   "missing field",
			body:   `{"asset_id":"show/thunderbirds/season/1/ep1","segment_index":0,"received_at":"2024-05-01T10:00:00Z"}`,
			status: http.StatusBadRequest,
			errMsg: "missing required field: cdn_url",
		},
	}

	handler := httpapi.NewHandler(store.New(), time.Unix(0, 0).UTC())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("expected %d, got %d body=%s", tt.status, rec.Code, rec.Body.String())
			}
			var resp map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode error: %v", err)
			}
			if resp["error"] != tt.errMsg {
				t.Fatalf("expected error %q, got %q", tt.errMsg, resp["error"])
			}
		})
	}
}

func TestManifestNotFound(t *testing.T) {
	handler := httpapi.NewHandler(store.New(), time.Unix(0, 0).UTC())
	req := httptest.NewRequest(http.MethodGet, "/manifest/show/nonexistent/season/1/ep99", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp["error"] != "asset not found" {
		t.Fatalf("unexpected error: %#v", resp)
	}
}

func TestHealth(t *testing.T) {
	startedAt := time.Now().Add(-2 * time.Second).UTC()
	handler := httpapi.NewHandler(store.New(), startedAt)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var resp model.HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if resp.Status != "ok" {
		t.Fatalf("expected ok status, got %#v", resp)
	}
	if resp.UptimeSeconds < 1 {
		t.Fatalf("expected uptime >= 1, got %d", resp.UptimeSeconds)
	}
}

func TestConcurrentMultiAssetAccess(t *testing.T) {
	st := store.New()
	handler := httpapi.NewHandler(st, time.Unix(0, 0).UTC())

	assets := []string{
		"show/thunderbirds/season/1/ep1",
		"show/scooby/season/1/ep4",
		"show/theoriginals/season/1/ep1",
	}

	const segmentsPerAsset = 50
	var wg sync.WaitGroup
	for _, assetID := range assets {
		assetID := assetID
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < segmentsPerAsset; i++ {
				body := []byte(`{"asset_id":"` + assetID + `","segment_index":` + strconv.Itoa(i) + `,"cdn_url":"https://cdn.example.com/` + assetID + `/seg` + strconv.Itoa(i) + `.ts","received_at":"2024-05-01T10:00:00Z"}`)
				req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusAccepted {
					t.Errorf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
					return
				}
			}
		}()
	}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodGet, "/manifest/"+assets[i%len(assets)], nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
		}(i)
	}

	wg.Wait()

	for _, assetID := range assets {
		req := httptest.NewRequest(http.MethodGet, "/manifest/"+assetID, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for %s, got %d body=%s", assetID, rec.Code, rec.Body.String())
		}
		var resp model.ManifestResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode manifest: %v", err)
		}
		if len(resp.Segments) != segmentsPerAsset {
			t.Fatalf("expected %d segments for %s, got %d", segmentsPerAsset, assetID, len(resp.Segments))
		}
	}
}

func BenchmarkManifestReadLargeAsset(b *testing.B) {
	st := store.New()
	assetID := "show/thunderbirds/season/1/ep3"
	for i := 0; i < 5000; i++ {
		st.UpsertSegment(assetID, store.Segment{
			Index:      i,
			CDNURL:     "https://cdn.example.com/thunderbirds/s1e3/seg" + strconv.Itoa(i) + ".ts",
			ReceivedAt: time.Unix(int64(i), 0).UTC(),
		})
	}

	handler := httpapi.NewHandler(st, time.Unix(0, 0).UTC())
	req := httptest.NewRequest(http.MethodGet, "/manifest/"+assetID, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			b.Fatalf("expected 200, got %d", rec.Code)
		}
		_, _ = io.ReadAll(rec.Body)
	}
}
