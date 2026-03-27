package queries

import (
	"context"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/ports"
)

type TagsQueryService struct {
	reader ports.BizTagReader
}

func NewTagsQueryService(reader ports.BizTagReader) TagsQueryService {
	return TagsQueryService{reader: reader}
}

func (s TagsQueryService) ListBizTags(ctx context.Context) ([]string, error) {
	return s.reader.ListBizTags(ctx)
}
