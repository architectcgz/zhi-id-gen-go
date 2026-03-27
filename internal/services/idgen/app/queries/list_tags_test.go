package queries

import (
	"context"
	"reflect"
	"testing"
)

type stubBizTagRepository struct {
	listBizTags func(ctx context.Context) ([]string, error)
}

func (s stubBizTagRepository) ListBizTags(ctx context.Context) ([]string, error) {
	return s.listBizTags(ctx)
}

func TestTagsQueryService_ListBizTags(t *testing.T) {
	service := NewTagsQueryService(stubBizTagRepository{
		listBizTags: func(_ context.Context) ([]string, error) {
			return []string{"order", "user"}, nil
		},
	})

	got, err := service.ListBizTags(context.Background())
	if err != nil {
		t.Fatalf("ListBizTags returned error: %v", err)
	}
	want := []string{"order", "user"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListBizTags got %v, want %v", got, want)
	}
}
