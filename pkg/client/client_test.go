package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestClient_DirectRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/id/snowflake":
			writeJSON(t, w, map[string]any{"code": 200, "message": "success", "data": int64(101)})
		case "/api/v1/id/segment/order":
			writeJSON(t, w, map[string]any{"code": 200, "message": "success", "data": int64(201)})
		case "/api/v1/id/snowflake/parse/101":
			writeJSON(t, w, map[string]any{
				"code":    200,
				"message": "success",
				"data": map[string]any{
					"id":           int64(101),
					"timestamp":    int64(1735689600001),
					"datacenterId": int64(1),
					"workerId":     int64(2),
					"sequence":     int64(3),
					"epoch":        int64(1735689600000),
				},
			})
		case "/api/v1/id/health":
			writeJSON(t, w, map[string]any{"code": 200, "message": "success", "data": map[string]any{"status": "UP"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := New(Config{
		ServerURL:     server.URL,
		BufferEnabled: false,
	})
	defer c.Close()

	snowflakeID, err := c.NextSnowflakeID()
	if err != nil {
		t.Fatalf("NextSnowflakeID returned error: %v", err)
	}
	if snowflakeID != 101 {
		t.Fatalf("snowflakeID = %d, want 101", snowflakeID)
	}

	segmentID, err := c.NextSegmentID("order")
	if err != nil {
		t.Fatalf("NextSegmentID returned error: %v", err)
	}
	if segmentID != 201 {
		t.Fatalf("segmentID = %d, want 201", segmentID)
	}

	info, err := c.ParseSnowflakeID(101)
	if err != nil {
		t.Fatalf("ParseSnowflakeID returned error: %v", err)
	}
	if info.WorkerID != 2 || info.DatacenterID != 1 {
		t.Fatalf("unexpected parse info: %+v", info)
	}

	if !c.IsHealthy() {
		t.Fatal("expected IsHealthy to return true")
	}
}

func TestClient_SnowflakeBufferReducesRoundTrips(t *testing.T) {
	var batchCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/id/snowflake/batch":
			batchCalls.Add(1)
			writeJSON(t, w, map[string]any{
				"code":    200,
				"message": "success",
				"data":    []int64{1, 2, 3, 4},
			})
		case "/api/v1/id/health":
			writeJSON(t, w, map[string]any{"code": 200, "message": "success", "data": map[string]any{"status": "UP"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := New(Config{
		ServerURL:       server.URL,
		BufferEnabled:   true,
		BufferSize:      4,
		RefillThreshold: 1,
		BatchFetchSize:  4,
		AsyncRefill:     false,
	})
	defer c.Close()

	for i := 1; i <= 3; i++ {
		id, err := c.NextSnowflakeID()
		if err != nil {
			t.Fatalf("NextSnowflakeID(%d) returned error: %v", i, err)
		}
		if int(id) != i {
			t.Fatalf("id[%d] = %d, want %d", i, id, i)
		}
	}

	if batchCalls.Load() != 1 {
		t.Fatalf("batchCalls = %d, want 1", batchCalls.Load())
	}
}

func TestClient_SegmentBufferUsesBizTagScopedQueues(t *testing.T) {
	var orderCalls atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/id/segment/order/batch":
			orderCalls.Add(1)
			writeJSON(t, w, map[string]any{
				"code":    200,
				"message": "success",
				"data":    []int64{10, 11, 12},
			})
		case "/api/v1/id/health":
			writeJSON(t, w, map[string]any{"code": 200, "message": "success", "data": map[string]any{"status": "UP"}})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	c := New(Config{
		ServerURL:       server.URL,
		BufferEnabled:   true,
		BufferSize:      3,
		RefillThreshold: 1,
		BatchFetchSize:  3,
		AsyncRefill:     false,
	})
	defer c.Close()

	first, err := c.NextSegmentID("order")
	if err != nil {
		t.Fatalf("first NextSegmentID returned error: %v", err)
	}
	second, err := c.NextSegmentID("order")
	if err != nil {
		t.Fatalf("second NextSegmentID returned error: %v", err)
	}
	if first != 10 || second != 11 {
		t.Fatalf("unexpected ids: first=%d second=%d", first, second)
	}
	if orderCalls.Load() != 1 {
		t.Fatalf("orderCalls = %d, want 1", orderCalls.Load())
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode json: %v", err)
	}
}
