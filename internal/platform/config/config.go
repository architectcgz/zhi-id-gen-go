package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddress               string
	ServiceName               string
	DatabaseURL               string
	SegmentTagRefreshInterval time.Duration
	Snowflake                 SnowflakeConfig
}

type SnowflakeConfig struct {
	WorkerID              int64
	DatacenterID          int64
	Epoch                 int64
	WorkerIDLeaseTimeout  time.Duration
	WorkerIDRenewInterval time.Duration
	BackupWorkerIDCount   int
}

func Load(serviceName string) Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8088"
	}

	return Config{
		HTTPAddress:               addr,
		ServiceName:               serviceName,
		DatabaseURL:               os.Getenv("DATABASE_URL"),
		SegmentTagRefreshInterval: parseDuration("SEGMENT_TAG_REFRESH_INTERVAL", 30*time.Second),
		Snowflake: SnowflakeConfig{
			WorkerID:              getEnvInt64("WORKER_ID", -1),
			DatacenterID:          getEnvInt64("DATACENTER_ID", 0),
			Epoch:                 getEnvInt64("SNOWFLAKE_EPOCH", 1735689600000),
			WorkerIDLeaseTimeout:  parseDuration("WORKER_ID_LEASE_TIMEOUT", 10*time.Minute),
			WorkerIDRenewInterval: parseDuration("WORKER_ID_RENEW_INTERVAL", 3*time.Minute),
			BackupWorkerIDCount:   getEnvInt("BACKUP_WORKER_ID_COUNT", 1),
		},
	}
}

func getEnvInt64(key string, fallback int64) int64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return value
}
