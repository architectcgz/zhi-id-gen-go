package queries

import "context"

type SnowflakeParseInfoView struct {
	ID           int64 `json:"id"`
	Timestamp    int64 `json:"timestamp"`
	DatacenterID int64 `json:"datacenterId"`
	WorkerID     int64 `json:"workerId"`
	Sequence     int64 `json:"sequence"`
	Epoch        int64 `json:"epoch"`
}

type SnowflakeInfoView struct {
	Initialized  bool   `json:"initialized"`
	WorkerID     *int   `json:"workerId"`
	DatacenterID *int   `json:"datacenterId"`
	Epoch        *int64 `json:"epoch"`
}

type SnowflakeQueryPort interface {
	ParseID(id int64) SnowflakeParseInfoView
	Info() SnowflakeInfoView
}

type SnowflakeQueryService struct {
	query SnowflakeQueryPort
}

func NewSnowflakeQueryService(query SnowflakeQueryPort) SnowflakeQueryService {
	return SnowflakeQueryService{query: query}
}

func (s SnowflakeQueryService) ParseSnowflakeID(_ context.Context, id int64) (SnowflakeParseInfoView, error) {
	return s.query.ParseID(id), nil
}

func (s SnowflakeQueryService) GetSnowflakeInfo(_ context.Context) (SnowflakeInfoView, error) {
	return s.query.Info(), nil
}
