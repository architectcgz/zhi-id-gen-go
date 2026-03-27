package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
	"github.com/architectcgz/zhi-id-gen-go/pkg/types"
)

func writeSuccess[T any](w http.ResponseWriter, data T) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(types.Success(data))
}

func writeError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	var codedErr *domain.Error
	if errors.As(err, &codedErr) {
		status := http.StatusInternalServerError
		switch codedErr.Code {
		case domain.ErrorInvalidArgument:
			status = http.StatusBadRequest
		case domain.ErrorBizTagNotExists:
			status = http.StatusNotFound
		case domain.ErrorCacheNotInitialized,
			domain.ErrorSegmentsNotReady,
			domain.ErrorWorkerIDUnavailable,
			domain.ErrorWorkerIDInvalid,
			domain.ErrorSegmentUpdateFailed,
			domain.ErrorServiceShuttingDown,
			domain.ErrorSnowflakeNotInitialized:
			status = http.StatusServiceUnavailable
		}

		w.WriteHeader(status)
		resp := types.Error(codedErr.Code.Code, codedErr.Code.Name, codedErr.Message)
		if len(codedErr.Extra) > 0 {
			resp = resp.WithExtra(codedErr.Extra)
		}
		_ = json.NewEncoder(w).Encode(resp)
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	_ = json.NewEncoder(w).Encode(types.Error(
		domain.ErrorInternalError.Code,
		domain.ErrorInternalError.Name,
		"Internal server error",
	))
}
