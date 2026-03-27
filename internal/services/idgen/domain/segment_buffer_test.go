package domain

import "testing"

func TestSegmentBuffer_NextIDTriggersPreloadAndSwitchesToNextSegment(t *testing.T) {
	buffer := NewSegmentBuffer("order")
	buffer.InitializeCurrent(SegmentAllocation{
		BizTag: "order",
		MaxID:  10,
		Step:   10,
	})

	first, err := buffer.NextID()
	if err != nil {
		t.Fatalf("first NextID returned error: %v", err)
	}
	if first.ID != 0 {
		t.Fatalf("first ID = %d, want 0", first.ID)
	}
	if first.ShouldLoadNext {
		t.Fatal("first ID should not trigger preload")
	}

	second, err := buffer.NextID()
	if err != nil {
		t.Fatalf("second NextID returned error: %v", err)
	}
	if !second.ShouldLoadNext {
		t.Fatal("second ID should trigger preload according to Java threshold behavior")
	}

	buffer.StoreNext(SegmentAllocation{
		BizTag: "order",
		MaxID:  20,
		Step:   10,
	})

	for i := 0; i < 8; i++ {
		if _, err := buffer.NextID(); err != nil {
			t.Fatalf("drain current segment: %v", err)
		}
	}

	switched, err := buffer.NextID()
	if err != nil {
		t.Fatalf("switch NextID returned error: %v", err)
	}
	if !switched.SwitchedSegment {
		t.Fatal("expected segment switch after current segment exhaustion")
	}
	if switched.ID != 10 {
		t.Fatalf("switched ID = %d, want 10", switched.ID)
	}
}

func TestSegmentBuffer_ExhaustedWithoutNextSegmentReturnsError(t *testing.T) {
	buffer := NewSegmentBuffer("order")
	buffer.InitializeCurrent(SegmentAllocation{
		BizTag: "order",
		MaxID:  2,
		Step:   2,
	})

	if _, err := buffer.NextID(); err != nil {
		t.Fatalf("first NextID returned error: %v", err)
	}
	if _, err := buffer.NextID(); err != nil {
		t.Fatalf("second NextID returned error: %v", err)
	}

	_, err := buffer.NextID()
	if err == nil {
		t.Fatal("expected error when current segment exhausted and next not ready")
	}
}

func TestSegmentBuffer_SnapshotMatchesJavaValueAndIdleSemantics(t *testing.T) {
	buffer := NewSegmentBuffer("order")
	buffer.InitializeCurrent(SegmentAllocation{
		BizTag: "order",
		MaxID:  100,
		Step:   10,
	})

	for i := 0; i < 2; i++ {
		if _, err := buffer.NextID(); err != nil {
			t.Fatalf("NextID returned error: %v", err)
		}
	}

	snapshot := buffer.Snapshot()
	if snapshot.CurrentSegment.Value != 92 {
		t.Fatalf("current value = %d, want 92", snapshot.CurrentSegment.Value)
	}
	if snapshot.CurrentSegment.Idle != 8 {
		t.Fatalf("current idle = %d, want 8", snapshot.CurrentSegment.Idle)
	}
}
