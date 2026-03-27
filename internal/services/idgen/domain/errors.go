package domain

import "fmt"

type ErrorCode struct {
	Code int
	Name string
}

var (
	ErrorInvalidArgument         = ErrorCode{Code: 40001, Name: "INVALID_ARGUMENT"}
	ErrorBizTagNotExists         = ErrorCode{Code: 40401, Name: "BIZ_TAG_NOT_EXISTS"}
	ErrorClockBackwards          = ErrorCode{Code: 50001, Name: "CLOCK_BACKWARDS"}
	ErrorIllegalState            = ErrorCode{Code: 50002, Name: "ILLEGAL_STATE"}
	ErrorCacheNotInitialized     = ErrorCode{Code: 50301, Name: "CACHE_NOT_INITIALIZED"}
	ErrorSegmentsNotReady        = ErrorCode{Code: 50302, Name: "SEGMENTS_NOT_READY"}
	ErrorWorkerIDUnavailable     = ErrorCode{Code: 50303, Name: "WORKER_ID_UNAVAILABLE"}
	ErrorWorkerIDInvalid         = ErrorCode{Code: 50304, Name: "WORKER_ID_INVALID"}
	ErrorSegmentUpdateFailed     = ErrorCode{Code: 50305, Name: "SEGMENT_UPDATE_FAILED"}
	ErrorServiceShuttingDown     = ErrorCode{Code: 50306, Name: "SERVICE_SHUTTING_DOWN"}
	ErrorSnowflakeNotInitialized = ErrorCode{Code: 50307, Name: "SNOWFLAKE_NOT_INITIALIZED"}
	ErrorInternalError           = ErrorCode{Code: 50000, Name: "INTERNAL_ERROR"}
)

type Error struct {
	Code    ErrorCode
	Message string
	Extra   map[string]any
}

func (e *Error) Error() string {
	return e.Message
}

func NewInvalidArgument(message string) error {
	return &Error{
		Code:    ErrorInvalidArgument,
		Message: message,
	}
}

func NewBizTagNotExists(bizTag string) error {
	return &Error{
		Code:    ErrorBizTagNotExists,
		Message: fmt.Sprintf("Business tag does not exist: %s", bizTag),
	}
}

func NewClockBackwards(offset int64) error {
	return &Error{
		Code:    ErrorClockBackwards,
		Message: fmt.Sprintf("Clock moved backwards by %dms", offset),
		Extra:   map[string]any{"offset": offset},
	}
}

func NewWorkerIDInvalid(message string) error {
	return &Error{
		Code:    ErrorWorkerIDInvalid,
		Message: message,
	}
}

func NewSegmentsNotReady(message string) error {
	return &Error{
		Code:    ErrorSegmentsNotReady,
		Message: message,
	}
}

func NewIllegalState(message string) error {
	return &Error{
		Code:    ErrorIllegalState,
		Message: message,
	}
}
