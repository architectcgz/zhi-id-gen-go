package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/commands"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/queries"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
)

type Handler struct {
	healthQuery       healthQueryService
	segmentCommands   segmentCommandService
	tagsQuery         tagsQueryService
	snowflakeCommands snowflakeCommandService
	snowflakeQueries  snowflakeQueryService
	segmentCacheQuery segmentCacheQueryService
}

type healthQueryService interface {
	GetHealth(ctx context.Context) (queries.HealthStatusView, error)
}

type segmentCommandService interface {
	GenerateSegmentID(ctx context.Context, bizTag string) (int64, error)
	GenerateBatchSegmentIDs(ctx context.Context, bizTag string, count int) ([]int64, error)
}

type tagsQueryService interface {
	ListBizTags(ctx context.Context) ([]string, error)
}

type snowflakeCommandService interface {
	GenerateSnowflakeID(ctx context.Context) (int64, error)
	GenerateBatchSnowflakeIDs(ctx context.Context, count int) ([]int64, error)
}

type snowflakeQueryService interface {
	ParseSnowflakeID(ctx context.Context, id int64) (queries.SnowflakeParseInfoView, error)
	GetSnowflakeInfo(ctx context.Context) (queries.SnowflakeInfoView, error)
}

type segmentCacheQueryService interface {
	GetCacheInfo(ctx context.Context, bizTag string) (queries.SegmentCacheInfoView, error)
}

func NewHandler(
	healthQuery healthQueryService,
	segmentCommands segmentCommandService,
	tagsQuery tagsQueryService,
	snowflakeCommands snowflakeCommandService,
	snowflakeQueries snowflakeQueryService,
	segmentCacheQuery segmentCacheQueryService,
) *Handler {
	return &Handler{
		healthQuery:       healthQuery,
		segmentCommands:   segmentCommands,
		tagsQuery:         tagsQuery,
		snowflakeCommands: snowflakeCommands,
		snowflakeQueries:  snowflakeQueries,
		segmentCacheQuery: segmentCacheQuery,
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)
	mux.HandleFunc("GET /api/v1/id/health", h.health)
	mux.HandleFunc("GET /api/v1/id/snowflake", h.generateSnowflakeID)
	mux.HandleFunc("GET /api/v1/id/snowflake/batch", h.generateBatchSnowflakeIDs)
	mux.HandleFunc("GET /api/v1/id/snowflake/parse/{id}", h.parseSnowflakeID)
	mux.HandleFunc("GET /api/v1/id/snowflake/info", h.getSnowflakeInfo)
	mux.HandleFunc("GET /api/v1/id/cache/{bizTag}", h.getSegmentCacheInfo)
	mux.HandleFunc("GET /api/v1/id/segment/{bizTag}", h.generateSegmentID)
	mux.HandleFunc("GET /api/v1/id/segment/{bizTag}/batch", h.generateBatchSegmentIDs)
	mux.HandleFunc("GET /api/v1/id/tags", h.listTags)
	return mux
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	health, err := h.healthQuery.GetHealth(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, health)
}

func (h *Handler) generateSegmentID(w http.ResponseWriter, r *http.Request) {
	id, err := h.segmentCommands.GenerateSegmentID(r.Context(), r.PathValue("bizTag"))
	if err != nil {
		writeError(w, err)
		return
	}

	writeSuccess(w, id)
}

func (h *Handler) generateBatchSegmentIDs(w http.ResponseWriter, r *http.Request) {
	count, err := parseBatchCount(r)
	if err != nil {
		writeError(w, err)
		return
	}

	ids, err := h.segmentCommands.GenerateBatchSegmentIDs(r.Context(), r.PathValue("bizTag"), count)
	if err != nil {
		writeError(w, err)
		return
	}

	writeSuccess(w, ids)
}

func (h *Handler) listTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.tagsQuery.ListBizTags(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	writeSuccess(w, tags)
}

func (h *Handler) generateSnowflakeID(w http.ResponseWriter, r *http.Request) {
	id, err := h.snowflakeCommands.GenerateSnowflakeID(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, id)
}

func (h *Handler) generateBatchSnowflakeIDs(w http.ResponseWriter, r *http.Request) {
	count, err := parseBatchCount(r)
	if err != nil {
		writeError(w, err)
		return
	}
	ids, err := h.snowflakeCommands.GenerateBatchSnowflakeIDs(r.Context(), count)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, ids)
}

func (h *Handler) parseSnowflakeID(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, domain.NewInvalidArgument("ID must be a positive number"))
		return
	}
	parsed, err := h.snowflakeQueries.ParseSnowflakeID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, parsed)
}

func (h *Handler) getSnowflakeInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.snowflakeQueries.GetSnowflakeInfo(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, info)
}

func (h *Handler) getSegmentCacheInfo(w http.ResponseWriter, r *http.Request) {
	if err := validateBizTag(r.PathValue("bizTag")); err != nil {
		writeError(w, err)
		return
	}

	info, err := h.segmentCacheQuery.GetCacheInfo(r.Context(), r.PathValue("bizTag"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w, info)
}

func parseBatchCount(r *http.Request) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("count"))
	if raw == "" {
		return 10, nil
	}

	count, err := strconv.Atoi(raw)
	if err != nil {
		return 0, commands.ValidateBatchCountError()
	}
	return count, nil
}

func validateBizTag(bizTag string) error {
	if strings.TrimSpace(bizTag) == "" {
		return domain.NewInvalidArgument("BizTag cannot be null or empty")
	}
	if len(bizTag) > 128 {
		return domain.NewInvalidArgument("BizTag length must not exceed 128 characters")
	}
	return nil
}
