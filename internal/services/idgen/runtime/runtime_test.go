package runtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/architectcgz/zhi-id-gen-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-id-gen-go/internal/platform/config"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/commands"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
)

type stubSegmentRangeRepository struct {
	listBizTags func(ctx context.Context) ([]string, error)
}

func (s stubSegmentRangeRepository) LoadSegmentRange(_ context.Context, _ string) (domain.SegmentAllocation, error) {
	return domain.SegmentAllocation{}, nil
}

func (s stubSegmentRangeRepository) ListBizTags(ctx context.Context) ([]string, error) {
	return s.listBizTags(ctx)
}

func TestBuild_AllowsStaticSnowflakeWithoutDatabase(t *testing.T) {
	app := &bootstrap.App{
		Config: config.Config{
			HTTPAddress: ":8088",
			ServiceName: "id-generator-service",
			Snowflake: config.SnowflakeConfig{
				WorkerID:     1,
				DatacenterID: 2,
				Epoch:        1735689600000,
			},
		},
	}

	runtimeOptions, err := Build(app)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/api/v1/id/health", nil)
	healthRec := httptest.NewRecorder()
	runtimeOptions.Handler.ServeHTTP(healthRec, healthReq)

	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status = %d, want %d", healthRec.Code, http.StatusOK)
	}

	var healthResp struct {
		Data struct {
			Status    string `json:"status"`
			Snowflake struct {
				Initialized bool `json:"initialized"`
			} `json:"snowflake"`
			Segment struct {
				Initialized bool `json:"initialized"`
			} `json:"segment"`
		} `json:"data"`
	}
	if err := json.Unmarshal(healthRec.Body.Bytes(), &healthResp); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	if healthResp.Data.Status != "DEGRADED" {
		t.Fatalf("health status = %s, want DEGRADED", healthResp.Data.Status)
	}
	if !healthResp.Data.Snowflake.Initialized {
		t.Fatal("snowflake initialized = false, want true")
	}
	if healthResp.Data.Segment.Initialized {
		t.Fatal("segment initialized = true, want false")
	}

	snowflakeReq := httptest.NewRequest(http.MethodGet, "/api/v1/id/snowflake", nil)
	snowflakeRec := httptest.NewRecorder()
	runtimeOptions.Handler.ServeHTTP(snowflakeRec, snowflakeReq)

	if snowflakeRec.Code != http.StatusOK {
		t.Fatalf("snowflake status = %d, want %d", snowflakeRec.Code, http.StatusOK)
	}
}

func TestSyncSegmentTagsWarmsUpAllocator(t *testing.T) {
	allocator := commands.NewCachedSegmentAllocator(
		stubSegmentRangeRepository{
			listBizTags: func(_ context.Context) ([]string, error) {
				return []string{"order", "user"}, nil
			},
		},
		nil,
	)

	if err := syncSegmentTags(context.Background(), stubSegmentRangeRepository{
		listBizTags: func(_ context.Context) ([]string, error) {
			return []string{"order", "user"}, nil
		},
	}, allocator); err != nil {
		t.Fatalf("syncSegmentTags returned error: %v", err)
	}

	got, err := allocator.ListBizTags(context.Background())
	if err != nil {
		t.Fatalf("ListBizTags returned error: %v", err)
	}
	want := []string{"order", "user"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListBizTags got %v, want %v", got, want)
	}
}
