package commands

import (
	"context"
	"reflect"
	"testing"
)

type stubSegmentRepository struct {
	allocateIDs func(ctx context.Context, bizTag string, count int) ([]int64, error)
}

func (s stubSegmentRepository) AllocateSegmentIDs(ctx context.Context, bizTag string, count int) ([]int64, error) {
	return s.allocateIDs(ctx, bizTag, count)
}

func TestSegmentCommandService_GenerateSegmentID(t *testing.T) {
	service := NewSegmentCommandService(stubSegmentRepository{
		allocateIDs: func(_ context.Context, bizTag string, count int) ([]int64, error) {
			if bizTag != "order" {
				t.Fatalf("unexpected bizTag: %s", bizTag)
			}
			if count != 1 {
				t.Fatalf("unexpected count: %d", count)
			}
			return []int64{1001}, nil
		},
	})

	got, err := service.GenerateSegmentID(context.Background(), "order")
	if err != nil {
		t.Fatalf("GenerateSegmentID returned error: %v", err)
	}
	if got != 1001 {
		t.Fatalf("GenerateSegmentID got %d, want 1001", got)
	}
}

func TestSegmentCommandService_GenerateBatchSegmentIDs(t *testing.T) {
	service := NewSegmentCommandService(stubSegmentRepository{
		allocateIDs: func(_ context.Context, bizTag string, count int) ([]int64, error) {
			if bizTag != "order" {
				t.Fatalf("unexpected bizTag: %s", bizTag)
			}
			if count != 3 {
				t.Fatalf("unexpected count: %d", count)
			}
			return []int64{2001, 2002, 2003}, nil
		},
	})

	got, err := service.GenerateBatchSegmentIDs(context.Background(), "order", 3)
	if err != nil {
		t.Fatalf("GenerateBatchSegmentIDs returned error: %v", err)
	}
	want := []int64{2001, 2002, 2003}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("GenerateBatchSegmentIDs got %v, want %v", got, want)
	}
}

func TestSegmentCommandService_RejectsInvalidBatchCount(t *testing.T) {
	service := NewSegmentCommandService(stubSegmentRepository{
		allocateIDs: func(_ context.Context, _ string, _ int) ([]int64, error) {
			t.Fatal("allocateIDs should not be called for invalid count")
			return nil, nil
		},
	})

	_, err := service.GenerateBatchSegmentIDs(context.Background(), "order", 0)
	if err == nil {
		t.Fatal("GenerateBatchSegmentIDs expected error for invalid count")
	}
}
