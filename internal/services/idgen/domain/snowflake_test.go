package domain

import "testing"

func TestSnowflakeGenerator_GenerateAndParse(t *testing.T) {
	generator := NewSnowflakeGenerator(1, 2, 1735689600000, func() int64 {
		return 1735689600123
	})

	id, err := generator.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID returned error: %v", err)
	}

	parsed := generator.ParseID(id)
	if parsed.WorkerID != 1 {
		t.Fatalf("parsed workerID = %d, want 1", parsed.WorkerID)
	}
	if parsed.DatacenterID != 2 {
		t.Fatalf("parsed datacenterID = %d, want 2", parsed.DatacenterID)
	}
	if parsed.Timestamp != 1735689600123 {
		t.Fatalf("parsed timestamp = %d, want 1735689600123", parsed.Timestamp)
	}
	if parsed.Epoch != 1735689600000 {
		t.Fatalf("parsed epoch = %d, want 1735689600000", parsed.Epoch)
	}
}

func TestSnowflakeGenerator_GenerateBatchInSameMillisecondIsMonotonic(t *testing.T) {
	generator := NewSnowflakeGenerator(1, 1, 1735689600000, func() int64 {
		return 1735689600999
	})

	first, err := generator.GenerateID()
	if err != nil {
		t.Fatalf("first GenerateID returned error: %v", err)
	}
	second, err := generator.GenerateID()
	if err != nil {
		t.Fatalf("second GenerateID returned error: %v", err)
	}

	if second <= first {
		t.Fatalf("expected second id > first id, got first=%d second=%d", first, second)
	}
}
