package queries

import "github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"

type SnowflakeQueryAdapter struct {
	generator *domain.SnowflakeGenerator
}

func NewSnowflakeQueryAdapter(generator *domain.SnowflakeGenerator) SnowflakeQueryAdapter {
	return SnowflakeQueryAdapter{generator: generator}
}

func (a SnowflakeQueryAdapter) ParseID(id int64) SnowflakeParseInfoView {
	parsed := a.generator.ParseID(id)
	return SnowflakeParseInfoView{
		ID:           parsed.ID,
		Timestamp:    parsed.Timestamp,
		DatacenterID: parsed.DatacenterID,
		WorkerID:     parsed.WorkerID,
		Sequence:     parsed.Sequence,
		Epoch:        parsed.Epoch,
	}
}

func (a SnowflakeQueryAdapter) Info() SnowflakeInfoView {
	workerID := int(a.generator.WorkerID())
	datacenterID := int(a.generator.DatacenterID())
	epoch := a.generator.Epoch()

	return SnowflakeInfoView{
		Initialized:  true,
		WorkerID:     &workerID,
		DatacenterID: &datacenterID,
		Epoch:        &epoch,
	}
}
