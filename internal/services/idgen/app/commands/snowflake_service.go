package commands

import (
	"context"
	"errors"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/queries"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
)

type snowflakeLeaseManager interface {
	IsWorkerIDValid() bool
	ConsumeBackupWorkerID(ctx context.Context) (int64, error)
}

type SnowflakeService struct {
	generator *domain.SnowflakeGenerator
	lease     snowflakeLeaseManager
}

func NewSnowflakeService(generator *domain.SnowflakeGenerator, lease snowflakeLeaseManager) SnowflakeService {
	return SnowflakeService{
		generator: generator,
		lease:     lease,
	}
}

func (s SnowflakeService) GenerateSnowflakeID(ctx context.Context) (int64, error) {
	if s.lease != nil && !s.lease.IsWorkerIDValid() {
		return 0, domain.NewWorkerIDInvalid("Worker ID is invalid, lease renewal failed")
	}

	id, err := s.generator.GenerateID()
	if err == nil {
		return id, nil
	}

	var clockErr *domain.ClockBackwardsError
	if !errors.As(err, &clockErr) || s.lease == nil {
		return 0, err
	}

	backupWorkerID, backupErr := s.lease.ConsumeBackupWorkerID(ctx)
	if backupErr != nil {
		return 0, domain.NewClockBackwards(clockErr.Offset)
	}

	s.generator.SwitchWorkerID(backupWorkerID)
	id, retryErr := s.generator.GenerateID()
	if retryErr != nil {
		var retryClockErr *domain.ClockBackwardsError
		if errors.As(retryErr, &retryClockErr) {
			return 0, domain.NewClockBackwards(retryClockErr.Offset)
		}
		return 0, retryErr
	}
	return id, nil
}

func (s SnowflakeService) GenerateBatchSnowflakeIDs(ctx context.Context, count int) ([]int64, error) {
	if err := validateBatchCount(count); err != nil {
		return nil, err
	}

	ids := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		id, err := s.GenerateSnowflakeID(ctx)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s SnowflakeService) ParseSnowflakeID(_ context.Context, id int64) (queries.SnowflakeParseInfoView, error) {
	parsed := s.generator.ParseID(id)
	return queries.SnowflakeParseInfoView{
		ID:           parsed.ID,
		Timestamp:    parsed.Timestamp,
		DatacenterID: parsed.DatacenterID,
		WorkerID:     parsed.WorkerID,
		Sequence:     parsed.Sequence,
		Epoch:        parsed.Epoch,
	}, nil
}

func (s SnowflakeService) GetSnowflakeInfo(_ context.Context) (queries.SnowflakeInfoView, error) {
	workerID := int(s.generator.WorkerID())
	datacenterID := int(s.generator.DatacenterID())
	epoch := s.generator.Epoch()

	return queries.SnowflakeInfoView{
		Initialized:  true,
		WorkerID:     &workerID,
		DatacenterID: &datacenterID,
		Epoch:        &epoch,
	}, nil
}
