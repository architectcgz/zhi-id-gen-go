package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
)

func TestWriteError_MapsCompatibilityErrorCodesToHTTPStatus(t *testing.T) {
	testCases := []struct {
		name          string
		err           error
		wantStatus    int
		wantErrorCode string
	}{
		{
			name:          "segments not ready",
			err:           domain.NewSegmentsNotReady("Both segments are not ready"),
			wantStatus:    http.StatusServiceUnavailable,
			wantErrorCode: "SEGMENTS_NOT_READY",
		},
		{
			name:          "illegal state",
			err:           domain.NewIllegalState("Internal state error"),
			wantStatus:    http.StatusInternalServerError,
			wantErrorCode: "ILLEGAL_STATE",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeError(rec, tc.err)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}

			var resp struct {
				ErrorCode string `json:"errorCode"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp.ErrorCode != tc.wantErrorCode {
				t.Fatalf("errorCode = %s, want %s", resp.ErrorCode, tc.wantErrorCode)
			}
		})
	}
}
