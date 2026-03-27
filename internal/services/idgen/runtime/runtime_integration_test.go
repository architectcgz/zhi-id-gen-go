//go:build integration

package runtime

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/architectcgz/zhi-id-gen-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-id-gen-go/internal/platform/config"
)

const testSnowflakeEpoch = 1735689600000

type postgresContainer struct {
	id       string
	hostPort string
	dsn      string
}

func TestRuntime_RefreshesSegmentTagsFromDatabase(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pg := startPostgresContainer(t, ctx)
	db := openIntegrationDB(t, pg.dsn)
	applySchema(t, db)

	runtimeOptions := buildIntegrationRuntime(t, db, pg.dsn, 100*time.Millisecond)
	server := httptest.NewServer(runtimeOptions.Handler)
	defer server.Close()

	assertTagsEventually(t, server.URL, []string{"default", "message", "order", "user"})

	if _, err := db.ExecContext(ctx, `INSERT INTO leaf_alloc (biz_tag, max_id, step, description) VALUES ('invoice', 1, 1000, 'Invoice ID')`); err != nil {
		t.Fatalf("insert invoice bizTag: %v", err)
	}

	assertTagsEventually(t, server.URL, []string{"default", "invoice", "message", "order", "user"})
	assertCacheEventually(t, server.URL, "invoice")
}

func TestRuntime_HealthRemainsAvailableAfterDatabaseOutage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pg := startPostgresContainer(t, ctx)
	db := openIntegrationDB(t, pg.dsn)
	applySchema(t, db)

	runtimeOptions := buildIntegrationRuntime(t, db, pg.dsn, time.Minute)
	server := httptest.NewServer(runtimeOptions.Handler)
	defer server.Close()

	stopPostgresContainer(t, pg.id)
	_ = db.Close()

	waitForCondition(t, 5*time.Second, func() bool {
		resp := fetchHealth(t, server.URL)
		return resp.Code == 200 && resp.Data.Status == "UP" && resp.Data.Segment.Initialized && resp.Data.Snowflake.Initialized
	}, "health stays UP after postgres outage")
}

func buildIntegrationRuntime(t *testing.T, db *sql.DB, dsn string, refreshInterval time.Duration) bootstrap.RuntimeOptions {
	t.Helper()

	app := &bootstrap.App{
		Config: config.Config{
			HTTPAddress:               ":8088",
			ServiceName:               "id-generator-service",
			DatabaseURL:               dsn,
			SegmentTagRefreshInterval: refreshInterval,
			Snowflake: config.SnowflakeConfig{
				WorkerID:     1,
				DatacenterID: 0,
				Epoch:        testSnowflakeEpoch,
			},
		},
		DB: db,
	}

	runtimeOptions, err := Build(app)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	t.Cleanup(func() {
		if runtimeOptions.Close != nil {
			_ = runtimeOptions.Close(context.Background())
		}
	})
	return runtimeOptions
}

func startPostgresContainer(t *testing.T, ctx context.Context) postgresContainer {
	t.Helper()

	cmd := exec.CommandContext(ctx,
		"docker", "run", "--rm", "-d",
		"-e", "POSTGRES_USER=postgres",
		"-e", "POSTGRES_PASSWORD=postgres",
		"-e", "POSTGRES_DB=idgen",
		"-p", "127.0.0.1::5432",
		"postgres:16-alpine",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("start postgres container: %v, output=%s", err, strings.TrimSpace(string(output)))
	}

	id := strings.TrimSpace(string(output))
	hostPort := dockerMappedPort(t, ctx, id)
	dsn := fmt.Sprintf("postgres://postgres:postgres@%s/idgen?sslmode=disable", hostPort)

	t.Cleanup(func() {
		stopPostgresContainer(t, id)
	})

	waitForCondition(t, 20*time.Second, func() bool {
		db, err := sql.Open("pgx", dsn)
		if err != nil {
			return false
		}
		defer db.Close()
		pingCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		return db.PingContext(pingCtx) == nil
	}, "postgres ready")

	return postgresContainer{
		id:       id,
		hostPort: hostPort,
		dsn:      dsn,
	}
}

func dockerMappedPort(t *testing.T, ctx context.Context, id string) string {
	t.Helper()

	cmd := exec.CommandContext(ctx, "docker", "port", id, "5432/tcp")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("inspect postgres port: %v, output=%s", err, strings.TrimSpace(string(output)))
	}

	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(output)), "0.0.0.0:"))
}

func stopPostgresContainer(t *testing.T, id string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", id)
	_, _ = cmd.CombinedOutput()
}

func openIntegrationDB(t *testing.T, dsn string) *sql.DB {
	t.Helper()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open integration db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func applySchema(t *testing.T, db *sql.DB) {
	t.Helper()

	schema, err := os.ReadFile("/home/azhi/workspace/projects/zhi-id-generator-go/sql/schema.sql")
	if err != nil {
		t.Fatalf("read schema.sql: %v", err)
	}

	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}
}

func assertTagsEventually(t *testing.T, baseURL string, want []string) {
	t.Helper()

	waitForCondition(t, 10*time.Second, func() bool {
		got, code := fetchTags(t, baseURL)
		return code == http.StatusOK && equalStrings(got, want)
	}, fmt.Sprintf("tags become %v", want))
}

func assertCacheEventually(t *testing.T, baseURL, bizTag string) {
	t.Helper()

	waitForCondition(t, 10*time.Second, func() bool {
		resp, code := fetchCacheInfo(t, baseURL, bizTag)
		return code == http.StatusOK &&
			resp.Data.Cached &&
			resp.Data.BufferInitialized != nil &&
			!*resp.Data.BufferInitialized
	}, "cache snapshot warmed up for "+bizTag)
}

func fetchTags(t *testing.T, baseURL string) ([]string, int) {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/v1/id/tags")
	if err != nil {
		return nil, 0
	}
	defer resp.Body.Close()

	var payload struct {
		Data []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode tags response: %v", err)
	}
	return payload.Data, resp.StatusCode
}

func fetchCacheInfo(t *testing.T, baseURL, bizTag string) (struct {
	Data struct {
		Cached            bool  `json:"cached"`
		BufferInitialized *bool `json:"bufferInitialized"`
	} `json:"data"`
}, int) {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/v1/id/cache/" + bizTag)
	if err != nil {
		return struct {
			Data struct {
				Cached            bool  `json:"cached"`
				BufferInitialized *bool `json:"bufferInitialized"`
			} `json:"data"`
		}{}, 0
	}
	defer resp.Body.Close()

	var payload struct {
		Data struct {
			Cached            bool  `json:"cached"`
			BufferInitialized *bool `json:"bufferInitialized"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode cache response: %v", err)
	}
	return payload, resp.StatusCode
}

func fetchHealth(t *testing.T, baseURL string) struct {
	Code int `json:"code"`
	Data struct {
		Status  string `json:"status"`
		Segment struct {
			Initialized bool `json:"initialized"`
		} `json:"segment"`
		Snowflake struct {
			Initialized bool `json:"initialized"`
		} `json:"snowflake"`
	} `json:"data"`
} {
	t.Helper()

	resp, err := http.Get(baseURL + "/api/v1/id/health")
	if err != nil {
		return struct {
			Code int `json:"code"`
			Data struct {
				Status  string `json:"status"`
				Segment struct {
					Initialized bool `json:"initialized"`
				} `json:"segment"`
				Snowflake struct {
					Initialized bool `json:"initialized"`
				} `json:"snowflake"`
			} `json:"data"`
		}{}
	}
	defer resp.Body.Close()

	var payload struct {
		Code int `json:"code"`
		Data struct {
			Status  string `json:"status"`
			Segment struct {
				Initialized bool `json:"initialized"`
			} `json:"segment"`
			Snowflake struct {
				Initialized bool `json:"initialized"`
			} `json:"snowflake"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode health response: %v", err)
	}
	return payload
}

func waitForCondition(t *testing.T, timeout time.Duration, condition func() bool, description string) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s: %s", timeout, description)
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
