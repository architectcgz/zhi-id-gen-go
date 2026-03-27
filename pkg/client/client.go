package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const apiBasePath = "/api/v1/id"

type Config struct {
	ServerURL       string
	ConnectTimeout  time.Duration
	ReadTimeout     time.Duration
	MaxRetries      int
	BufferSize      int
	RefillThreshold int
	BatchFetchSize  int
	AsyncRefill     bool
	BufferEnabled   bool
}

func DefaultConfig() Config {
	return Config{
		ServerURL:       "http://localhost:8088",
		ConnectTimeout:  5 * time.Second,
		ReadTimeout:     5 * time.Second,
		MaxRetries:      3,
		BufferSize:      100,
		RefillThreshold: 20,
		BatchFetchSize:  50,
		AsyncRefill:     true,
		BufferEnabled:   true,
	}
}

type ErrorCode string

const (
	ErrorUnknown          ErrorCode = "UNKNOWN"
	ErrorConnectionFailed ErrorCode = "CONNECTION_FAILED"
	ErrorServerError      ErrorCode = "SERVER_ERROR"
	ErrorInvalidResponse  ErrorCode = "INVALID_RESPONSE"
	ErrorBizTagNotFound   ErrorCode = "BIZ_TAG_NOT_FOUND"
	ErrorClientClosed     ErrorCode = "CLIENT_CLOSED"
	ErrorInvalidArgument  ErrorCode = "INVALID_ARGUMENT"
)

type Error struct {
	Code    ErrorCode
	Message string
	Err     error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type SnowflakeIDInfo struct {
	ID           int64 `json:"id"`
	Timestamp    int64 `json:"timestamp"`
	DatacenterID int64 `json:"datacenterId"`
	WorkerID     int64 `json:"workerId"`
	Sequence     int64 `json:"sequence"`
	Epoch        int64 `json:"epoch"`
}

type Client struct {
	config     Config
	httpClient *http.Client

	mu                 sync.Mutex
	closed             bool
	snowflakeBuf       []int64
	snowflakeRefilling bool
	segmentBufs        map[string][]int64
	segmentRefilling   map[string]bool
}

func New(cfg Config) *Client {
	defaults := DefaultConfig()
	if cfg.ServerURL == "" {
		cfg.ServerURL = defaults.ServerURL
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = defaults.ConnectTimeout
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = defaults.ReadTimeout
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = defaults.BufferSize
	}
	if cfg.RefillThreshold < 0 || cfg.RefillThreshold >= cfg.BufferSize {
		cfg.RefillThreshold = defaults.RefillThreshold
	}
	if cfg.BatchFetchSize <= 0 {
		cfg.BatchFetchSize = defaults.BatchFetchSize
	}

	return &Client{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.ReadTimeout,
			Transport: &http.Transport{
				DialContext:           (&net.Dialer{Timeout: cfg.ConnectTimeout}).DialContext,
				ResponseHeaderTimeout: cfg.ReadTimeout,
			},
		},
		segmentBufs:      make(map[string][]int64),
		segmentRefilling: make(map[string]bool),
	}
}

func (c *Client) NextSnowflakeID() (int64, error) {
	if err := c.ensureOpen(); err != nil {
		return 0, err
	}
	if !c.config.BufferEnabled {
		return c.fetchSnowflakeID()
	}

	c.mu.Lock()
	if len(c.snowflakeBuf) > 0 {
		id := c.snowflakeBuf[0]
		c.snowflakeBuf = c.snowflakeBuf[1:]
		needRefill := len(c.snowflakeBuf) < c.config.RefillThreshold
		c.mu.Unlock()
		if needRefill {
			c.triggerSnowflakeRefill()
		}
		return id, nil
	}
	c.mu.Unlock()

	if err := c.refillSnowflakeBuffer(); err != nil {
		return 0, err
	}
	return c.NextSnowflakeID()
}

func (c *Client) NextSnowflakeIDs(count int) ([]int64, error) {
	if err := validateCount(count); err != nil {
		return nil, err
	}
	if !c.config.BufferEnabled || count > c.config.BufferSize {
		return c.fetchSnowflakeIDs(count)
	}

	ids := make([]int64, 0, count)
	for len(ids) < count {
		id, err := c.NextSnowflakeID()
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *Client) NextSegmentID(bizTag string) (int64, error) {
	if err := validateBizTag(bizTag); err != nil {
		return 0, err
	}
	if err := c.ensureOpen(); err != nil {
		return 0, err
	}
	if !c.config.BufferEnabled {
		return c.fetchSegmentID(bizTag)
	}

	c.mu.Lock()
	buf := c.segmentBufs[bizTag]
	if len(buf) > 0 {
		id := buf[0]
		c.segmentBufs[bizTag] = buf[1:]
		needRefill := len(c.segmentBufs[bizTag]) < c.config.RefillThreshold
		c.mu.Unlock()
		if needRefill {
			c.triggerSegmentRefill(bizTag)
		}
		return id, nil
	}
	c.mu.Unlock()

	if err := c.refillSegmentBuffer(bizTag); err != nil {
		return 0, err
	}
	return c.NextSegmentID(bizTag)
}

func (c *Client) NextSegmentIDs(bizTag string, count int) ([]int64, error) {
	if err := validateBizTag(bizTag); err != nil {
		return nil, err
	}
	if err := validateCount(count); err != nil {
		return nil, err
	}
	if !c.config.BufferEnabled || count > c.config.BufferSize {
		return c.fetchSegmentIDs(bizTag, count)
	}

	ids := make([]int64, 0, count)
	for len(ids) < count {
		id, err := c.NextSegmentID(bizTag)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (c *Client) ParseSnowflakeID(id int64) (SnowflakeIDInfo, error) {
	if err := c.ensureOpen(); err != nil {
		return SnowflakeIDInfo{}, err
	}
	var info SnowflakeIDInfo
	if err := c.getJSON(fmt.Sprintf("%s/snowflake/parse/%d", apiBasePath, id), &info); err != nil {
		return SnowflakeIDInfo{}, err
	}
	return info, nil
}

func (c *Client) IsHealthy() bool {
	if err := c.ensureOpen(); err != nil {
		return false
	}
	var data struct {
		Status string `json:"status"`
	}
	if err := c.getJSON(apiBasePath+"/health", &data); err != nil {
		return false
	}
	return data.Status == "UP"
}

func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.snowflakeBuf = nil
	c.segmentBufs = make(map[string][]int64)
	c.segmentRefilling = make(map[string]bool)
}

func (c *Client) ensureOpen() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return &Error{Code: ErrorClientClosed, Message: "ID Generator client is closed"}
	}
	return nil
}

func (c *Client) triggerSnowflakeRefill() {
	if !c.config.AsyncRefill {
		return
	}
	c.mu.Lock()
	if c.snowflakeRefilling {
		c.mu.Unlock()
		return
	}
	c.snowflakeRefilling = true
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			c.snowflakeRefilling = false
			c.mu.Unlock()
		}()
		_ = c.refillSnowflakeBuffer()
	}()
}

func (c *Client) triggerSegmentRefill(bizTag string) {
	if !c.config.AsyncRefill {
		return
	}
	c.mu.Lock()
	if c.segmentRefilling[bizTag] {
		c.mu.Unlock()
		return
	}
	c.segmentRefilling[bizTag] = true
	c.mu.Unlock()

	go func() {
		defer func() {
			c.mu.Lock()
			c.segmentRefilling[bizTag] = false
			c.mu.Unlock()
		}()
		_ = c.refillSegmentBuffer(bizTag)
	}()
}

func (c *Client) refillSnowflakeBuffer() error {
	c.mu.Lock()
	toFetch := c.config.BufferSize - len(c.snowflakeBuf)
	c.mu.Unlock()
	if toFetch <= 0 {
		return nil
	}
	if toFetch > c.config.BatchFetchSize {
		toFetch = c.config.BatchFetchSize
	}
	ids, err := c.fetchSnowflakeIDs(toFetch)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.snowflakeBuf = append(c.snowflakeBuf, ids...)
	if len(c.snowflakeBuf) > c.config.BufferSize {
		c.snowflakeBuf = c.snowflakeBuf[:c.config.BufferSize]
	}
	return nil
}

func (c *Client) refillSegmentBuffer(bizTag string) error {
	c.mu.Lock()
	toFetch := c.config.BufferSize - len(c.segmentBufs[bizTag])
	c.mu.Unlock()
	if toFetch <= 0 {
		return nil
	}
	if toFetch > c.config.BatchFetchSize {
		toFetch = c.config.BatchFetchSize
	}
	ids, err := c.fetchSegmentIDs(bizTag, toFetch)
	if err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.segmentBufs[bizTag] = append(c.segmentBufs[bizTag], ids...)
	if len(c.segmentBufs[bizTag]) > c.config.BufferSize {
		c.segmentBufs[bizTag] = c.segmentBufs[bizTag][:c.config.BufferSize]
	}
	return nil
}

func (c *Client) fetchSnowflakeID() (int64, error) {
	var id int64
	if err := c.getJSON(apiBasePath+"/snowflake", &id); err != nil {
		return 0, err
	}
	return id, nil
}

func (c *Client) fetchSnowflakeIDs(count int) ([]int64, error) {
	var ids []int64
	if err := c.getJSON(apiBasePath+"/snowflake/batch?count="+strconv.Itoa(count), &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (c *Client) fetchSegmentID(bizTag string) (int64, error) {
	var id int64
	if err := c.getJSON(apiBasePath+"/segment/"+url.PathEscape(bizTag), &id); err != nil {
		return 0, err
	}
	return id, nil
}

func (c *Client) fetchSegmentIDs(bizTag string, count int) ([]int64, error) {
	var ids []int64
	path := apiBasePath + "/segment/" + url.PathEscape(bizTag) + "/batch?count=" + strconv.Itoa(count)
	if err := c.getJSON(path, &ids); err != nil {
		return nil, err
	}
	return ids, nil
}

func (c *Client) getJSON(path string, out any) error {
	fullURL := strings.TrimRight(c.config.ServerURL, "/") + path
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		req, err := http.NewRequest(http.MethodGet, fullURL, nil)
		if err != nil {
			return &Error{Code: ErrorInvalidArgument, Message: "invalid request URL", Err: err}
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		err = decodeResponse(resp, out)
		_ = resp.Body.Close()
		if err == nil {
			return nil
		}

		var clientErr *Error
		if ok := As(err, &clientErr); ok && (clientErr.Code == ErrorServerError || clientErr.Code == ErrorBizTagNotFound || clientErr.Code == ErrorInvalidArgument) {
			return err
		}
		lastErr = err
	}

	return &Error{
		Code:    ErrorConnectionFailed,
		Message: "failed to connect to ID Generator server",
		Err:     lastErr,
	}
}

func decodeResponse(resp *http.Response, out any) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Error{Code: ErrorInvalidResponse, Message: "failed to read response", Err: err}
	}

	var wrapper struct {
		Code      int             `json:"code"`
		Message   string          `json:"message"`
		ErrorCode string          `json:"errorCode"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return &Error{Code: ErrorInvalidResponse, Message: "invalid JSON response", Err: err}
	}

	if resp.StatusCode >= 400 || (wrapper.Code != 0 && wrapper.Code != 200 && wrapper.Code >= 400) {
		code := ErrorServerError
		switch wrapper.ErrorCode {
		case "BIZ_TAG_NOT_EXISTS":
			code = ErrorBizTagNotFound
		case "INVALID_ARGUMENT":
			code = ErrorInvalidArgument
		}
		return &Error{Code: code, Message: wrapper.Message}
	}

	if out == nil {
		return nil
	}
	if len(wrapper.Data) == 0 || string(wrapper.Data) == "null" {
		return &Error{Code: ErrorInvalidResponse, Message: "missing data field"}
	}
	if err := json.Unmarshal(wrapper.Data, out); err != nil {
		return &Error{Code: ErrorInvalidResponse, Message: "failed to decode data field", Err: err}
	}
	return nil
}

func validateBizTag(bizTag string) error {
	if strings.TrimSpace(bizTag) == "" {
		return &Error{Code: ErrorInvalidArgument, Message: "bizTag cannot be null or empty"}
	}
	return nil
}

func validateCount(count int) error {
	if count < 1 || count > 1000 {
		return &Error{Code: ErrorInvalidArgument, Message: "count must be between 1 and 1000"}
	}
	return nil
}

func As(err error, target any) bool {
	return errorsAs(err, target)
}
