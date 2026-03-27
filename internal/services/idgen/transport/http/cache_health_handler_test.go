package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/queries"
)

type stubSegmentCacheQuery struct {
	getCacheInfo func(ctx context.Context, bizTag string) (queries.SegmentCacheInfoView, error)
}

func (s stubSegmentCacheQuery) GetCacheInfo(ctx context.Context, bizTag string) (queries.SegmentCacheInfoView, error) {
	return s.getCacheInfo(ctx, bizTag)
}

func TestHandler_GetSegmentCacheInfo(t *testing.T) {
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
			generateSingle: func(_ context.Context) (int64, error) { return 1, nil },
			generateBatch:  func(_ context.Context, _ int) ([]int64, error) { return []int64{1}, nil },
		},
		stubSnowflakeQueries{
			parse: func(_ context.Context, _ int64) (queries.SnowflakeParseInfoView, error) {
				return queries.SnowflakeParseInfoView{}, nil
			},
			info: func(_ context.Context) (queries.SnowflakeInfoView, error) { return queries.SnowflakeInfoView{}, nil },
		},
		stubSegmentCacheQuery{
			getCacheInfo: func(_ context.Context, bizTag string) (queries.SegmentCacheInfoView, error) {
				return queries.SegmentCacheInfoView{BizTag: bizTag, Cached: true, Initialized: true}, nil
			},
		},
	).Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/id/cache/order", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var resp struct {
		Data queries.SegmentCacheInfoView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.BizTag != "order" || !resp.Data.Cached {
		t.Fatalf("unexpected data: %+v", resp.Data)
	}
}

func TestHandler_GetHealthDetailed(t *testing.T) {
	handler := NewHandler(
		stubHealthQuery{getHealth: func(_ context.Context) (queries.HealthStatusView, error) {
			return queries.HealthStatusView{
				Status:  "UP",
				Service: "id-generator-service",
				Segment: queries.SegmentHealthView{Initialized: true, BizTagCount: 2},
			}, nil
		}},
		stubSegmentCommands{
			generateSingle: func(_ context.Context, _ string) (int64, error) { return 1, nil },
			generateBatch:  func(_ context.Context, _ string, _ int) ([]int64, error) { return []int64{1}, nil },
		},
		stubTagsQuery{listBizTags: func(_ context.Context) ([]string, error) { return []string{"order"}, nil }},
		stubSnowflakeCommands{
			generateSingle: func(_ context.Context) (int64, error) { return 1, nil },
			generateBatch:  func(_ context.Context, _ int) ([]int64, error) { return []int64{1}, nil },
		},
		stubSnowflakeQueries{
			parse: func(_ context.Context, _ int64) (queries.SnowflakeParseInfoView, error) {
				return queries.SnowflakeParseInfoView{}, nil
			},
			info: func(_ context.Context) (queries.SnowflakeInfoView, error) { return queries.SnowflakeInfoView{}, nil },
		},
		stubSegmentCacheQuery{
			getCacheInfo: func(_ context.Context, _ string) (queries.SegmentCacheInfoView, error) {
				return queries.SegmentCacheInfoView{}, nil
			},
		},
	).Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/id/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp struct {
		Data queries.HealthStatusView `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Data.Segment.BizTagCount != 2 {
		t.Fatalf("unexpected health data: %+v", resp.Data)
	}
}

func TestHandler_GetSegmentCacheInfoRejectsTooLongBizTag(t *testing.T) {
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
			generateSingle: func(_ context.Context) (int64, error) { return 1, nil },
			generateBatch:  func(_ context.Context, _ int) ([]int64, error) { return []int64{1}, nil },
		},
		stubSnowflakeQueries{
			parse: func(_ context.Context, _ int64) (queries.SnowflakeParseInfoView, error) {
				return queries.SnowflakeParseInfoView{}, nil
			},
			info: func(_ context.Context) (queries.SnowflakeInfoView, error) { return queries.SnowflakeInfoView{}, nil },
		},
		stubSegmentCacheQuery{
			getCacheInfo: func(_ context.Context, _ string) (queries.SegmentCacheInfoView, error) {
				t.Fatal("GetCacheInfo should not be called")
				return queries.SegmentCacheInfoView{}, nil
			},
		},
	).Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/id/cache/"+strings.Repeat("x", 129), nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
