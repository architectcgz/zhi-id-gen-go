package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/queries"
)

type stubSnowflakeCommands struct {
	generateSingle func(ctx context.Context) (int64, error)
	generateBatch  func(ctx context.Context, count int) ([]int64, error)
}

func (s stubSnowflakeCommands) GenerateSnowflakeID(ctx context.Context) (int64, error) {
	return s.generateSingle(ctx)
}

func (s stubSnowflakeCommands) GenerateBatchSnowflakeIDs(ctx context.Context, count int) ([]int64, error) {
	return s.generateBatch(ctx, count)
}

type stubSnowflakeQueries struct {
	parse func(ctx context.Context, id int64) (queries.SnowflakeParseInfoView, error)
	info  func(ctx context.Context) (queries.SnowflakeInfoView, error)
}

func (s stubSnowflakeQueries) ParseSnowflakeID(ctx context.Context, id int64) (queries.SnowflakeParseInfoView, error) {
	return s.parse(ctx, id)
}

func (s stubSnowflakeQueries) GetSnowflakeInfo(ctx context.Context) (queries.SnowflakeInfoView, error) {
	return s.info(ctx)
}

func TestHandler_GenerateSnowflakeID(t *testing.T) {
	handler := NewHandler(
		stubHealthQuery{getHealth: func(_ context.Context) (queries.HealthStatusView, error) {
			return queries.HealthStatusView{}, nil
		}},
		stubSegmentCommands{
			generateSingle: func(_ context.Context, _ string) (int64, error) { return 1, nil },
			generateBatch:  func(_ context.Context, _ string, _ int) ([]int64, error) { return []int64{1}, nil },
		},
		stubTagsQuery{listBizTags: func(_ context.Context) ([]string, error) { return []string{"order"}, nil }},
		stubSnowflakeCommands{
			generateSingle: func(_ context.Context) (int64, error) { return 123, nil },
			generateBatch:  func(_ context.Context, _ int) ([]int64, error) { return []int64{123}, nil },
		},
		stubSnowflakeQueries{
			parse: func(_ context.Context, _ int64) (queries.SnowflakeParseInfoView, error) {
				return queries.SnowflakeParseInfoView{}, nil
			},
			info: func(_ context.Context) (queries.SnowflakeInfoView, error) {
				return queries.SnowflakeInfoView{}, nil
			},
		},
		stubSegmentCacheQuery{
			getCacheInfo: func(_ context.Context, _ string) (queries.SegmentCacheInfoView, error) {
				return queries.SegmentCacheInfoView{}, nil
			},
		},
	).Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/id/snowflake", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_GenerateBatchSnowflakeIDs(t *testing.T) {
	handler := NewHandler(
		stubHealthQuery{getHealth: func(_ context.Context) (queries.HealthStatusView, error) {
			return queries.HealthStatusView{}, nil
		}},
		stubSegmentCommands{
			generateSingle: func(_ context.Context, _ string) (int64, error) { return 1, nil },
			generateBatch:  func(_ context.Context, _ string, _ int) ([]int64, error) { return []int64{1}, nil },
		},
		stubTagsQuery{listBizTags: func(_ context.Context) ([]string, error) { return []string{"order"}, nil }},
		stubSnowflakeCommands{
			generateSingle: func(_ context.Context) (int64, error) { return 123, nil },
			generateBatch: func(_ context.Context, count int) ([]int64, error) {
				if count != 3 {
					t.Fatalf("unexpected count: %d", count)
				}
				return []int64{1, 2, 3}, nil
			},
		},
		stubSnowflakeQueries{
			parse: func(_ context.Context, _ int64) (queries.SnowflakeParseInfoView, error) {
				return queries.SnowflakeParseInfoView{}, nil
			},
			info: func(_ context.Context) (queries.SnowflakeInfoView, error) {
				return queries.SnowflakeInfoView{}, nil
			},
		},
		stubSegmentCacheQuery{
			getCacheInfo: func(_ context.Context, _ string) (queries.SegmentCacheInfoView, error) {
				return queries.SegmentCacheInfoView{}, nil
			},
		},
	).Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/id/snowflake/batch?count=3", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp struct {
		Data []int64 `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !reflect.DeepEqual(resp.Data, []int64{1, 2, 3}) {
		t.Fatalf("unexpected data: %v", resp.Data)
	}
}

func TestHandler_GenerateBatchSnowflakeIDsUsesDefaultCount(t *testing.T) {
	handler := NewHandler(
		stubHealthQuery{getHealth: func(_ context.Context) (queries.HealthStatusView, error) {
			return queries.HealthStatusView{}, nil
		}},
		stubSegmentCommands{
			generateSingle: func(_ context.Context, _ string) (int64, error) { return 1, nil },
			generateBatch:  func(_ context.Context, _ string, _ int) ([]int64, error) { return []int64{1}, nil },
		},
		stubTagsQuery{listBizTags: func(_ context.Context) ([]string, error) { return []string{"order"}, nil }},
		stubSnowflakeCommands{
			generateSingle: func(_ context.Context) (int64, error) { return 123, nil },
			generateBatch: func(_ context.Context, count int) ([]int64, error) {
				if count != 10 {
					t.Fatalf("unexpected count: %d", count)
				}
				return []int64{1, 2}, nil
			},
		},
		stubSnowflakeQueries{
			parse: func(_ context.Context, _ int64) (queries.SnowflakeParseInfoView, error) {
				return queries.SnowflakeParseInfoView{}, nil
			},
			info: func(_ context.Context) (queries.SnowflakeInfoView, error) {
				return queries.SnowflakeInfoView{}, nil
			},
		},
		stubSegmentCacheQuery{
			getCacheInfo: func(_ context.Context, _ string) (queries.SegmentCacheInfoView, error) {
				return queries.SegmentCacheInfoView{}, nil
			},
		},
	).Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/id/snowflake/batch", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_ParseSnowflakeIDAndInfo(t *testing.T) {
	handler := NewHandler(
		stubHealthQuery{getHealth: func(_ context.Context) (queries.HealthStatusView, error) {
			return queries.HealthStatusView{}, nil
		}},
		stubSegmentCommands{
			generateSingle: func(_ context.Context, _ string) (int64, error) { return 1, nil },
			generateBatch:  func(_ context.Context, _ string, _ int) ([]int64, error) { return []int64{1}, nil },
		},
		stubTagsQuery{listBizTags: func(_ context.Context) ([]string, error) { return []string{"order"}, nil }},
		stubSnowflakeCommands{
			generateSingle: func(_ context.Context) (int64, error) { return 123, nil },
			generateBatch:  func(_ context.Context, _ int) ([]int64, error) { return []int64{123}, nil },
		},
		stubSnowflakeQueries{
			parse: func(_ context.Context, id int64) (queries.SnowflakeParseInfoView, error) {
				return queries.SnowflakeParseInfoView{ID: id, WorkerID: 1, DatacenterID: 2}, nil
			},
			info: func(_ context.Context) (queries.SnowflakeInfoView, error) {
				return queries.SnowflakeInfoView{Initialized: true, WorkerID: intPtr(1), DatacenterID: intPtr(2)}, nil
			},
		},
		stubSegmentCacheQuery{
			getCacheInfo: func(_ context.Context, _ string) (queries.SegmentCacheInfoView, error) {
				return queries.SegmentCacheInfoView{}, nil
			},
		},
	).Routes()

	parseReq := httptest.NewRequest(http.MethodGet, "/api/v1/id/snowflake/parse/123", nil)
	parseRec := httptest.NewRecorder()
	handler.ServeHTTP(parseRec, parseReq)
	if parseRec.Code != http.StatusOK {
		t.Fatalf("parse status = %d, want %d", parseRec.Code, http.StatusOK)
	}

	infoReq := httptest.NewRequest(http.MethodGet, "/api/v1/id/snowflake/info", nil)
	infoRec := httptest.NewRecorder()
	handler.ServeHTTP(infoRec, infoReq)
	if infoRec.Code != http.StatusOK {
		t.Fatalf("info status = %d, want %d", infoRec.Code, http.StatusOK)
	}
}

func intPtr(v int) *int { return &v }
