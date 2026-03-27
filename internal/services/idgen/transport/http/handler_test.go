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

type stubHealthQuery struct {
	getHealth func(ctx context.Context) (queries.HealthStatusView, error)
}

func (s stubHealthQuery) GetHealth(ctx context.Context) (queries.HealthStatusView, error) {
	return s.getHealth(ctx)
}

type stubSegmentCommands struct {
	generateSingle func(ctx context.Context, bizTag string) (int64, error)
	generateBatch  func(ctx context.Context, bizTag string, count int) ([]int64, error)
}

func (s stubSegmentCommands) GenerateSegmentID(ctx context.Context, bizTag string) (int64, error) {
	return s.generateSingle(ctx, bizTag)
}

func (s stubSegmentCommands) GenerateBatchSegmentIDs(ctx context.Context, bizTag string, count int) ([]int64, error) {
	return s.generateBatch(ctx, bizTag, count)
}

type stubTagsQuery struct {
	listBizTags func(ctx context.Context) ([]string, error)
}

func (s stubTagsQuery) ListBizTags(ctx context.Context) ([]string, error) {
	return s.listBizTags(ctx)
}

func TestHandler_GenerateSegmentID(t *testing.T) {
	handler := NewHandler(
		stubHealthQuery{getHealth: func(_ context.Context) (queries.HealthStatusView, error) {
			return queries.HealthStatusView{}, nil
		}},
		stubSegmentCommands{
			generateSingle: func(_ context.Context, bizTag string) (int64, error) {
				if bizTag != "order" {
					t.Fatalf("unexpected input: bizTag=%s", bizTag)
				}
				return 1001, nil
			},
			generateBatch: func(_ context.Context, _ string, _ int) ([]int64, error) {
				t.Fatal("GenerateBatchSegmentIDs should not be called")
				return nil, nil
			},
		},
		stubTagsQuery{
			listBizTags: func(_ context.Context) ([]string, error) {
				return []string{"order"}, nil
			},
		},
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/id/segment/order", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Code int   `json:"code"`
		Data int64 `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Code != 200 {
		t.Fatalf("response code = %d, want 200", resp.Code)
	}
	if resp.Data != 1001 {
		t.Fatalf("response data = %d, want 1001", resp.Data)
	}
}

func TestHandler_GenerateBatchSegmentIDs(t *testing.T) {
	handler := NewHandler(
		stubHealthQuery{getHealth: func(_ context.Context) (queries.HealthStatusView, error) {
			return queries.HealthStatusView{}, nil
		}},
		stubSegmentCommands{
			generateSingle: func(_ context.Context, _ string) (int64, error) {
				t.Fatal("GenerateSegmentID should not be called")
				return 0, nil
			},
			generateBatch: func(_ context.Context, bizTag string, count int) ([]int64, error) {
				if bizTag != "order" || count != 3 {
					t.Fatalf("unexpected input: bizTag=%s count=%d", bizTag, count)
				}
				return []int64{2001, 2002, 2003}, nil
			},
		},
		stubTagsQuery{
			listBizTags: func(_ context.Context) ([]string, error) {
				return []string{"order"}, nil
			},
		},
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/id/segment/order/batch?count=3", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Code int     `json:"code"`
		Data []int64 `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := []int64{2001, 2002, 2003}
	if !reflect.DeepEqual(resp.Data, want) {
		t.Fatalf("response data = %v, want %v", resp.Data, want)
	}
}

func TestHandler_GenerateBatchSegmentIDsUsesDefaultCount(t *testing.T) {
	handler := NewHandler(
		stubHealthQuery{getHealth: func(_ context.Context) (queries.HealthStatusView, error) {
			return queries.HealthStatusView{}, nil
		}},
		stubSegmentCommands{
			generateSingle: func(_ context.Context, _ string) (int64, error) {
				t.Fatal("GenerateSegmentID should not be called")
				return 0, nil
			},
			generateBatch: func(_ context.Context, bizTag string, count int) ([]int64, error) {
				if bizTag != "order" || count != 10 {
					t.Fatalf("unexpected input: bizTag=%s count=%d", bizTag, count)
				}
				return []int64{0, 1}, nil
			},
		},
		stubTagsQuery{
			listBizTags: func(_ context.Context) ([]string, error) {
				return []string{"order"}, nil
			},
		},
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/id/segment/order/batch", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestHandler_ListTags(t *testing.T) {
	handler := NewHandler(
		stubHealthQuery{getHealth: func(_ context.Context) (queries.HealthStatusView, error) {
			return queries.HealthStatusView{}, nil
		}},
		stubSegmentCommands{
			generateSingle: func(_ context.Context, _ string) (int64, error) {
				return 1001, nil
			},
			generateBatch: func(_ context.Context, _ string, _ int) ([]int64, error) {
				return []int64{1001}, nil
			},
		},
		stubTagsQuery{
			listBizTags: func(_ context.Context) ([]string, error) {
				return []string{"order", "user"}, nil
			},
		},
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

	req := httptest.NewRequest(http.MethodGet, "/api/v1/id/tags", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp struct {
		Code int      `json:"code"`
		Data []string `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := []string{"order", "user"}
	if !reflect.DeepEqual(resp.Data, want) {
		t.Fatalf("response data = %v, want %v", resp.Data, want)
	}
}
