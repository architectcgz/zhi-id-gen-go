package types

import "maps"

type ApiResponse[T any] struct {
	Code      int            `json:"code"`
	Message   string         `json:"message"`
	Data      *T             `json:"data,omitempty"`
	ErrorCode string         `json:"errorCode,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

func Success[T any](data T) ApiResponse[T] {
	return ApiResponse[T]{
		Code:    200,
		Message: "success",
		Data:    &data,
	}
}

func Error(code int, errorCode, message string) ApiResponse[any] {
	return ApiResponse[any]{
		Code:      code,
		Message:   message,
		ErrorCode: errorCode,
	}
}

func (r ApiResponse[T]) WithExtra(extra map[string]any) ApiResponse[T] {
	if len(extra) == 0 {
		return r
	}

	r.Extra = maps.Clone(extra)
	return r
}
