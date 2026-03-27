package domain

import (
	"fmt"
	"sync"
	"time"
)

const (
	snowflakeWorkerIDBits     = 5
	snowflakeDatacenterIDBits = 5
	snowflakeSequenceBits     = 12

	snowflakeMaxWorkerID     = -1 ^ (-1 << snowflakeWorkerIDBits)
	snowflakeMaxDatacenterID = -1 ^ (-1 << snowflakeDatacenterIDBits)
	snowflakeMaxSequence     = -1 ^ (-1 << snowflakeSequenceBits)

	snowflakeWorkerIDShift     = snowflakeSequenceBits
	snowflakeDatacenterIDShift = snowflakeSequenceBits + snowflakeWorkerIDBits
	snowflakeTimestampShift    = snowflakeSequenceBits + snowflakeWorkerIDBits + snowflakeDatacenterIDBits
)

type SnowflakeParseResult struct {
	ID           int64
	Timestamp    int64
	DatacenterID int64
	WorkerID     int64
	Sequence     int64
	Epoch        int64
}

type ClockBackwardsError struct {
	Offset int64
}

func (e *ClockBackwardsError) Error() string {
	return fmt.Sprintf("Clock moved backwards by %dms", e.Offset)
}

type SnowflakeGenerator struct {
	workerID     int64
	datacenterID int64
	epoch        int64
	now          func() int64

	mu            sync.Mutex
	lastTimestamp int64
	sequence      int64
}

func NewSnowflakeGenerator(workerID, datacenterID int64, epoch int64, now func() int64) *SnowflakeGenerator {
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &SnowflakeGenerator{
		workerID:      workerID,
		datacenterID:  datacenterID,
		epoch:         epoch,
		now:           now,
		lastTimestamp: -1,
	}
}

func (g *SnowflakeGenerator) GenerateID() (int64, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	timestamp := g.now()
	if timestamp < g.lastTimestamp {
		return 0, &ClockBackwardsError{Offset: g.lastTimestamp - timestamp}
	}

	if timestamp == g.lastTimestamp {
		g.sequence = (g.sequence + 1) & snowflakeMaxSequence
		if g.sequence == 0 {
			for timestamp <= g.lastTimestamp {
				timestamp = g.now()
			}
		}
	} else {
		g.sequence = 0
	}

	g.lastTimestamp = timestamp
	id := ((timestamp - g.epoch) << snowflakeTimestampShift) |
		(g.datacenterID << snowflakeDatacenterIDShift) |
		(g.workerID << snowflakeWorkerIDShift) |
		g.sequence
	return id, nil
}

func (g *SnowflakeGenerator) ParseID(id int64) SnowflakeParseResult {
	timestampPart := (id >> snowflakeTimestampShift) + g.epoch
	datacenterID := (id >> snowflakeDatacenterIDShift) & snowflakeMaxDatacenterID
	workerID := (id >> snowflakeWorkerIDShift) & snowflakeMaxWorkerID
	sequence := id & snowflakeMaxSequence

	return SnowflakeParseResult{
		ID:           id,
		Timestamp:    timestampPart,
		DatacenterID: datacenterID,
		WorkerID:     workerID,
		Sequence:     sequence,
		Epoch:        g.epoch,
	}
}

func (g *SnowflakeGenerator) WorkerID() int64 {
	return g.workerID
}

func (g *SnowflakeGenerator) DatacenterID() int64 {
	return g.datacenterID
}

func (g *SnowflakeGenerator) Epoch() int64 {
	return g.epoch
}

func (g *SnowflakeGenerator) SwitchWorkerID(workerID int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.workerID = workerID
	g.sequence = 0
	g.lastTimestamp = -1
}
