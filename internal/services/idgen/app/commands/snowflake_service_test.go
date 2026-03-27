package commands

import (
	"context"
	"testing"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
)

type stubLeaseManager struct {
	valid         bool
	consumeBackup func(ctx context.Context) (int64, error)
}

func (s stubLeaseManager) IsWorkerIDValid() bool {
	return s.valid
}

func (s stubLeaseManager) ConsumeBackupWorkerID(ctx context.Context) (int64, error) {
	return s.consumeBackup(ctx)
}

func TestSnowflakeService_SwitchesBackupWorkerOnClockBackwards(t *testing.T) {
	timestamps := []int64{1735689600100, 1735689600090, 1735689600110}
	index := 0
	generator := domain.NewSnowflakeGenerator(1, 0, 1735689600000, func() int64 {
		ts := timestamps[index]
		index++
		return ts
	})
	service := NewSnowflakeService(
		generator,
		stubLeaseManager{
			valid: true,
			consumeBackup: func(_ context.Context) (int64, error) {
				return 2, nil
			},
		},
	)

	first, err := service.GenerateSnowflakeID(context.Background())
	if err != nil {
		t.Fatalf("first GenerateSnowflakeID returned error: %v", err)
	}
	second, err := service.GenerateSnowflakeID(context.Background())
	if err != nil {
		t.Fatalf("second GenerateSnowflakeID returned error: %v", err)
	}

	if second == first {
		t.Fatalf("expected different ids after worker switch, got first=%d second=%d", first, second)
	}
	info, err := service.GetSnowflakeInfo(context.Background())
	if err != nil {
		t.Fatalf("GetSnowflakeInfo returned error: %v", err)
	}
	if info.WorkerID == nil || *info.WorkerID != 2 {
		t.Fatalf("worker id after switch = %v, want 2", info.WorkerID)
	}
}
