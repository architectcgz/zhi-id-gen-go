package queries

import (
	"context"
	"testing"
)

type stubSegmentCacheObserver struct {
	getSnapshot func(bizTag string) (SegmentCacheInfoView, bool)
}

func (s stubSegmentCacheObserver) GetSegmentCacheInfo(bizTag string) (SegmentCacheInfoView, bool) {
	return s.getSnapshot(bizTag)
}

type stubHealthTagsReader struct {
	listBizTags func(ctx context.Context) ([]string, error)
	initialized func() bool
}

func (s stubHealthTagsReader) ListBizTags(ctx context.Context) ([]string, error) {
	return s.listBizTags(ctx)
}

func (s stubHealthTagsReader) IsInitialized() bool {
	if s.initialized == nil {
		return true
	}
	return s.initialized()
}

type stubHealthSnowflakeInfo struct {
	getInfo func(ctx context.Context) (SnowflakeInfoView, error)
}

func (s stubHealthSnowflakeInfo) GetSnowflakeInfo(ctx context.Context) (SnowflakeInfoView, error) {
	return s.getInfo(ctx)
}

func TestSegmentCacheQueryService_GetCacheInfo(t *testing.T) {
	service := NewSegmentCacheQueryService(stubSegmentCacheObserver{
		getSnapshot: func(bizTag string) (SegmentCacheInfoView, bool) {
			if bizTag != "order" {
				t.Fatalf("unexpected bizTag: %s", bizTag)
			}
			return SegmentCacheInfoView{
				BizTag:         "order",
				Initialized:    true,
				Cached:         true,
				CurrentPos:     intPtr(0),
				CurrentSegment: &SegmentStateView{Value: 11, Max: 20, Step: 10, Idle: 10},
			}, true
		},
	}, func() bool { return true })

	got, err := service.GetCacheInfo(context.Background(), "order")
	if err != nil {
		t.Fatalf("GetCacheInfo returned error: %v", err)
	}
	if !got.Cached {
		t.Fatal("expected cached=true")
	}
	if got.CurrentSegment == nil || got.CurrentSegment.Value != 11 {
		t.Fatalf("unexpected current segment: %+v", got.CurrentSegment)
	}
}

func TestHealthQueryService_GetHealth(t *testing.T) {
	service := NewHealthQueryService(
		"id-generator-service",
		stubHealthTagsReader{
			listBizTags: func(_ context.Context) ([]string, error) {
				return []string{"order", "user"}, nil
			},
			initialized: func() bool { return false },
		},
		stubHealthSnowflakeInfo{
			getInfo: func(_ context.Context) (SnowflakeInfoView, error) {
				return SnowflakeInfoView{
					Initialized:  true,
					WorkerID:     intPtr(3),
					DatacenterID: intPtr(1),
				}, nil
			},
		},
	)

	got, err := service.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth returned error: %v", err)
	}
	if got.Status != "DEGRADED" {
		t.Fatalf("status = %s, want DEGRADED", got.Status)
	}
	if got.Service != "id-generator-service" {
		t.Fatalf("service = %s, want id-generator-service", got.Service)
	}
	if got.Segment.BizTagCount != 2 {
		t.Fatalf("bizTagCount = %d, want 2", got.Segment.BizTagCount)
	}
	if got.Segment.Initialized {
		t.Fatal("segment initialized = true, want false")
	}
	if got.Snowflake.WorkerID == nil || *got.Snowflake.WorkerID != 3 {
		t.Fatalf("workerID = %v, want 3", got.Snowflake.WorkerID)
	}
}

func TestHealthQueryService_GetHealthDegradesWhenSnowflakeLeaseIsInvalid(t *testing.T) {
	service := NewHealthQueryService(
		"id-generator-service",
		stubHealthTagsReader{
			listBizTags: func(_ context.Context) ([]string, error) {
				return []string{"order"}, nil
			},
			initialized: func() bool { return true },
		},
		stubHealthSnowflakeInfo{
			getInfo: func(_ context.Context) (SnowflakeInfoView, error) {
				return SnowflakeInfoView{
					Initialized:   true,
					WorkerID:      intPtr(3),
					DatacenterID:  intPtr(1),
					WorkerIDValid: boolQueryPtr(false),
				}, nil
			},
		},
	)

	got, err := service.GetHealth(context.Background())
	if err != nil {
		t.Fatalf("GetHealth returned error: %v", err)
	}
	if got.Status != "DEGRADED" {
		t.Fatalf("status = %s, want DEGRADED", got.Status)
	}
	if got.Snowflake.WorkerIDValid == nil || *got.Snowflake.WorkerIDValid {
		t.Fatalf("workerIdValid = %v, want false", got.Snowflake.WorkerIDValid)
	}
}

func TestSegmentCacheQueryService_GetCacheInfoReturnsSegmentInitializationWhenNotCached(t *testing.T) {
	service := NewSegmentCacheQueryService(
		stubSegmentCacheObserver{
			getSnapshot: func(_ string) (SegmentCacheInfoView, bool) {
				return SegmentCacheInfoView{}, false
			},
		},
		func() bool { return false },
	)

	got, err := service.GetCacheInfo(context.Background(), "order")
	if err != nil {
		t.Fatalf("GetCacheInfo returned error: %v", err)
	}
	if got.Cached {
		t.Fatal("cached = true, want false")
	}
	if got.Initialized {
		t.Fatal("initialized = true, want false")
	}
}

func TestSegmentCacheQueryService_GetCacheInfoReturnsWarmupCacheSnapshot(t *testing.T) {
	service := NewSegmentCacheQueryService(
		stubSegmentCacheObserver{
			getSnapshot: func(_ string) (SegmentCacheInfoView, bool) {
				return SegmentCacheInfoView{
					BizTag:            "order",
					Cached:            true,
					BufferInitialized: boolQueryPtr(false),
				}, true
			},
		},
		func() bool { return true },
	)

	got, err := service.GetCacheInfo(context.Background(), "order")
	if err != nil {
		t.Fatalf("GetCacheInfo returned error: %v", err)
	}
	if !got.Initialized {
		t.Fatal("initialized = false, want true")
	}
	if got.BufferInitialized == nil || *got.BufferInitialized {
		t.Fatalf("bufferInitialized = %v, want false", got.BufferInitialized)
	}
}

func intPtr(v int) *int { return &v }

func boolQueryPtr(v bool) *bool { return &v }
