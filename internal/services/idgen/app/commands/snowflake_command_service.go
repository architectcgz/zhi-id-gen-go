package commands

import (
	"context"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
)

type SnowflakeGeneratorPort interface {
	GenerateID() (int64, error)
}

type SnowflakeCommandService struct {
	generator SnowflakeGeneratorPort
}

func NewSnowflakeCommandService(generator SnowflakeGeneratorPort) SnowflakeCommandService {
	return SnowflakeCommandService{generator: generator}
}

func (s SnowflakeCommandService) GenerateSnowflakeID(_ context.Context) (int64, error) {
	return s.generator.GenerateID()
}

func (s SnowflakeCommandService) GenerateBatchSnowflakeIDs(_ context.Context, count int) ([]int64, error) {
	if err := validateBatchCount(count); err != nil {
		return nil, err
	}

	ids := make([]int64, 0, count)
	for i := 0; i < count; i++ {
		id, err := s.generator.GenerateID()
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

var _ SnowflakeGeneratorPort = (*domain.SnowflakeGenerator)(nil)
