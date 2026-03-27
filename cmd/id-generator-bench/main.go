package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"time"

	"github.com/architectcgz/zhi-id-gen-go/internal/platform/benchtool"
	"github.com/architectcgz/zhi-id-gen-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-id-gen-go/internal/platform/config"
	idruntime "github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/runtime"
)

type benchmarkResult struct {
	Mode       string `json:"mode"`
	URL        string `json:"url"`
	Workers    int    `json:"workers"`
	BatchCount int    `json:"batchCount,omitempty"`
	DurationMS int64  `json:"durationMs"`
	Requests   int    `json:"requests"`
	TotalIDs   int    `json:"totalIds"`
	UniqueIDs  int    `json:"uniqueIds"`
	Duplicates int    `json:"duplicates"`
}

type snowflakeResponse struct {
	Code int   `json:"code"`
	Data int64 `json:"data"`
}

type snowflakeBatchResponse struct {
	Code int     `json:"code"`
	Data []int64 `json:"data"`
}

func main() {
	mode := flag.String("mode", "http", "benchmark mode: http or batch")
	url := flag.String("url", "", "server base URL; empty means use in-process local server")
	workers := flag.Int("workers", runtime.GOMAXPROCS(0)*4, "number of concurrent workers")
	duration := flag.Duration("duration", time.Second, "benchmark duration")
	batchCount := flag.Int("count", 1000, "batch count when mode=batch")
	flag.Parse()

	baseURL, cleanup, err := resolveBaseURL(*url)
	if err != nil {
		panic(err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	client := &http.Client{Transport: &http.Transport{
		MaxIdleConns:        2048,
		MaxIdleConnsPerHost: 2048,
		MaxConnsPerHost:     2048,
	}}

	result, err := runBenchmark(client, baseURL, *mode, *workers, *duration, *batchCount)
	if err != nil {
		panic(err)
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(output))
}

func resolveBaseURL(url string) (string, func(), error) {
	if url != "" {
		return url, nil, nil
	}

	rt, err := idruntime.Build(&bootstrap.App{Config: config.Config{
		HTTPAddress: ":8088",
		ServiceName: "id-generator-service",
		Snowflake: config.SnowflakeConfig{
			WorkerID:     1,
			DatacenterID: 1,
			Epoch:        1735689600000,
		},
	}})
	if err != nil {
		return "", nil, err
	}

	server := httptest.NewServer(rt.Handler)
	cleanup := func() {
		server.Close()
		if rt.Close != nil {
			_ = rt.Close(nil)
		}
	}
	return server.URL, cleanup, nil
}

func runBenchmark(client *http.Client, baseURL, mode string, workers int, duration time.Duration, batchCount int) (benchmarkResult, error) {
	deadline := time.Now().Add(duration)
	results := make([][]int64, workers)
	requests := make([]int, workers)
	errs := make([]error, workers)

	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		idx := i
		go func() {
			defer wg.Done()
			local := make([]int64, 0, 4096)
			reqCount := 0

			for time.Now().Before(deadline) {
				var ids []int64
				var err error
				switch mode {
				case "http":
					var id int64
					id, err = fetchSnowflakeID(client, baseURL+"/api/v1/id/snowflake")
					if err == nil {
						ids = []int64{id}
					}
				case "batch":
					ids, err = fetchSnowflakeBatch(client, fmt.Sprintf("%s/api/v1/id/snowflake/batch?count=%d", baseURL, batchCount))
				default:
					err = fmt.Errorf("unsupported mode %q", mode)
				}

				if err != nil {
					errs[idx] = err
					return
				}
				local = append(local, ids...)
				reqCount++
			}

			results[idx] = local
			requests[idx] = reqCount
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return benchmarkResult{}, err
		}
	}

	totalRequests := 0
	totalIDs := 0
	all := make([]int64, 0)
	for i, batch := range results {
		totalRequests += requests[i]
		totalIDs += len(batch)
		all = append(all, batch...)
	}

	duplicates := benchtool.CountDuplicates(all)
	result := benchmarkResult{
		Mode:       mode,
		URL:        baseURL,
		Workers:    workers,
		BatchCount: batchCount,
		DurationMS: duration.Milliseconds(),
		Requests:   totalRequests,
		TotalIDs:   totalIDs,
		UniqueIDs:  totalIDs - duplicates,
		Duplicates: duplicates,
	}
	return result, nil
}

func fetchSnowflakeID(client *http.Client, url string) (int64, error) {
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var payload snowflakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return 0, err
	}
	if resp.StatusCode != http.StatusOK || payload.Code != 200 {
		return 0, fmt.Errorf("unexpected response: status=%d code=%d", resp.StatusCode, payload.Code)
	}
	return payload.Data, nil
}

func fetchSnowflakeBatch(client *http.Client, url string) ([]int64, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var payload snowflakeBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK || payload.Code != 200 {
		return nil, fmt.Errorf("unexpected response: status=%d code=%d", resp.StatusCode, payload.Code)
	}
	return payload.Data, nil
}
