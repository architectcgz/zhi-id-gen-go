package queries

import (
	"context"
	"time"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
)

type SegmentStateView struct {
	Value int64 `json:"value"`
	Max   int64 `json:"max"`
	Step  int   `json:"step"`
	Idle  int64 `json:"idle"`
}

type SegmentCacheInfoView struct {
	BizTag             string            `json:"bizTag"`
	Initialized        bool              `json:"initialized"`
	Cached             bool              `json:"cached"`
	BufferInitialized  *bool             `json:"bufferInitialized,omitempty"`
	CurrentPos         *int              `json:"currentPos,omitempty"`
	NextReady          *bool             `json:"nextReady,omitempty"`
	LoadingNextSegment *bool             `json:"loadingNextSegment,omitempty"`
	MinStep            *int              `json:"minStep,omitempty"`
	UpdateTimestamp    *int64            `json:"updateTimestamp,omitempty"`
	CurrentSegment     *SegmentStateView `json:"currentSegment,omitempty"`
	NextSegment        *SegmentStateView `json:"nextSegment,omitempty"`
}

type SegmentHealthView struct {
	Initialized bool `json:"initialized"`
	BizTagCount int  `json:"bizTagCount"`
}

type SnowflakeHealthView struct {
	Initialized  bool `json:"initialized"`
	WorkerID     *int `json:"workerId"`
	DatacenterID *int `json:"datacenterId"`
}

type HealthStatusView struct {
	Status    string              `json:"status"`
	Service   string              `json:"service"`
	Timestamp int64               `json:"timestamp"`
	Segment   SegmentHealthView   `json:"segment"`
	Snowflake SnowflakeHealthView `json:"snowflake"`
}

type segmentCacheObserver interface {
	GetSegmentCacheInfo(bizTag string) (SegmentCacheInfoView, bool)
}

type healthTagsReader interface {
	ListBizTags(ctx context.Context) ([]string, error)
	IsInitialized() bool
}

type healthSnowflakeInfo interface {
	GetSnowflakeInfo(ctx context.Context) (SnowflakeInfoView, error)
}

type HealthQueryService struct {
	serviceName string
	tagsReader  healthTagsReader
	snowflake   healthSnowflakeInfo
}

func NewHealthQueryService(serviceName string, tagsReader healthTagsReader, snowflake healthSnowflakeInfo) HealthQueryService {
	return HealthQueryService{
		serviceName: serviceName,
		tagsReader:  tagsReader,
		snowflake:   snowflake,
	}
}

func (s HealthQueryService) GetHealth(ctx context.Context) (HealthStatusView, error) {
	tags, err := s.tagsReader.ListBizTags(ctx)
	if err != nil {
		return HealthStatusView{}, err
	}
	snowflakeInfo, err := s.snowflake.GetSnowflakeInfo(ctx)
	if err != nil {
		return HealthStatusView{}, err
	}

	status := "UP"
	segmentInitialized := s.tagsReader.IsInitialized()
	if !segmentInitialized || !snowflakeInfo.Initialized {
		status = "DEGRADED"
	}

	return HealthStatusView{
		Status:    status,
		Service:   s.serviceName,
		Timestamp: time.Now().UnixMilli(),
		Segment: SegmentHealthView{
			Initialized: segmentInitialized,
			BizTagCount: len(tags),
		},
		Snowflake: SnowflakeHealthView{
			Initialized:  snowflakeInfo.Initialized,
			WorkerID:     snowflakeInfo.WorkerID,
			DatacenterID: snowflakeInfo.DatacenterID,
		},
	}, nil
}

type SegmentCacheQueryService struct {
	observer    segmentCacheObserver
	initialized func() bool
}

func NewSegmentCacheQueryService(observer segmentCacheObserver, initialized func() bool) SegmentCacheQueryService {
	if initialized == nil {
		initialized = func() bool { return true }
	}
	return SegmentCacheQueryService{
		observer:    observer,
		initialized: initialized,
	}
}

func (s SegmentCacheQueryService) GetCacheInfo(_ context.Context, bizTag string) (SegmentCacheInfoView, error) {
	if bizTag == "" {
		return SegmentCacheInfoView{}, domain.NewInvalidArgument("BizTag cannot be null or empty")
	}

	snapshot, ok := s.observer.GetSegmentCacheInfo(bizTag)
	if ok {
		snapshot.Initialized = s.initialized()
		return snapshot, nil
	}

	return SegmentCacheInfoView{
		BizTag:      bizTag,
		Initialized: s.initialized(),
		Cached:      false,
	}, nil
}

func ToSegmentStateView(snapshot domain.SegmentStateSnapshot) *SegmentStateView {
	return &SegmentStateView{
		Value: snapshot.Value,
		Max:   snapshot.Max,
		Step:  snapshot.Step,
		Idle:  snapshot.Idle,
	}
}
