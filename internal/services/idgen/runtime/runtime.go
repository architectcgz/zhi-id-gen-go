package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/architectcgz/zhi-id-gen-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/commands"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/queries"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
	segmentpostgres "github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/infra/postgres"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/ports"
	transporthttp "github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/transport/http"
)

type unavailableSegmentCommandService struct{}

func (unavailableSegmentCommandService) GenerateSegmentID(context.Context, string) (int64, error) {
	return 0, &domain.Error{
		Code:    domain.ErrorCacheNotInitialized,
		Message: "Segment service not initialized",
	}
}

func (unavailableSegmentCommandService) GenerateBatchSegmentIDs(context.Context, string, int) ([]int64, error) {
	return nil, &domain.Error{
		Code:    domain.ErrorCacheNotInitialized,
		Message: "Segment service not initialized",
	}
}

func Build(app *bootstrap.App) (bootstrap.RuntimeOptions, error) {
	if app.DB == nil && app.Config.Snowflake.WorkerID < 0 {
		return bootstrap.RuntimeOptions{}, errors.New("DATABASE_URL is required when segment mode or DB worker mode is enabled")
	}

	snowflakeService, closeFn, err := buildSnowflakeService(app)
	if err != nil {
		return bootstrap.RuntimeOptions{}, err
	}
	segmentAllocator := commands.NewCachedSegmentAllocator(nil, nil)

	if app.DB == nil {
		healthQuery := queries.NewHealthQueryService(app.Config.ServiceName, segmentAllocator, snowflakeService)
		cacheQuery := queries.NewSegmentCacheQueryService(segmentAllocator, segmentAllocator.IsInitialized)
		handler := transporthttp.NewHandler(
			healthQuery,
			unavailableSegmentCommandService{},
			queries.NewTagsQueryService(segmentAllocator),
			snowflakeService,
			snowflakeService,
			cacheQuery,
		)
		return bootstrap.RuntimeOptions{
			Handler: handler.Routes(),
			Close:   closeFn,
		}, nil
	}

	segmentRepository := segmentpostgres.NewSegmentRepository(app.DB)
	segmentAllocator = commands.NewCachedSegmentAllocator(segmentRepository, nil)
	if err := syncSegmentTags(context.Background(), segmentRepository, segmentAllocator); err != nil {
		return bootstrap.RuntimeOptions{}, err
	}
	refreshStop := startSegmentTagRefresher(segmentRepository, segmentAllocator, app.Config.SegmentTagRefreshInterval)

	healthQuery := queries.NewHealthQueryService(app.Config.ServiceName, segmentAllocator, snowflakeService)
	cacheQuery := queries.NewSegmentCacheQueryService(segmentAllocator, segmentAllocator.IsInitialized)
	handler := transporthttp.NewHandler(
		healthQuery,
		commands.NewSegmentCommandService(segmentAllocator),
		queries.NewTagsQueryService(segmentAllocator),
		snowflakeService,
		snowflakeService,
		cacheQuery,
	)
	return bootstrap.RuntimeOptions{
		Handler: handler.Routes(),
		Close: func(ctx context.Context) error {
			if refreshStop != nil {
				refreshStop()
			}
			return closeFn(ctx)
		},
	}, nil
}

func syncSegmentTags(ctx context.Context, reader ports.BizTagReader, allocator *commands.CachedSegmentAllocator) error {
	bizTags, err := reader.ListBizTags(ctx)
	if err != nil {
		return err
	}
	allocator.Warmup(bizTags)
	return nil
}

func startSegmentTagRefresher(reader ports.BizTagReader, allocator *commands.CachedSegmentAllocator, interval time.Duration) func() {
	if interval <= 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = syncSegmentTags(context.Background(), reader, allocator)
			}
		}
	}()
	return cancel
}

func buildSnowflakeService(app *bootstrap.App) (commands.SnowflakeService, func(context.Context) error, error) {
	generatorClose := func(context.Context) error { return nil }

	if app.Config.Snowflake.WorkerID >= 0 {
		generator := domain.NewSnowflakeGenerator(
			app.Config.Snowflake.WorkerID,
			app.Config.Snowflake.DatacenterID,
			app.Config.Snowflake.Epoch,
			nil,
		)
		return commands.NewSnowflakeService(generator, nil), generatorClose, nil
	}

	instanceID, err := buildInstanceID(app.Config.HTTPAddress)
	if err != nil {
		return commands.SnowflakeService{}, nil, err
	}

	leaseManager, err := commands.NewDBWorkerLeaseManager(
		context.Background(),
		segmentpostgres.NewWorkerLeaseStore(app.DB),
		instanceID,
		app.Config.Snowflake.WorkerIDLeaseTimeout,
		app.Config.Snowflake.WorkerIDRenewInterval,
		app.Config.Snowflake.BackupWorkerIDCount,
	)
	if err != nil {
		return commands.SnowflakeService{}, nil, err
	}

	generator := domain.NewSnowflakeGenerator(
		leaseManager.PrimaryWorkerID(),
		app.Config.Snowflake.DatacenterID,
		app.Config.Snowflake.Epoch,
		nil,
	)

	return commands.NewSnowflakeService(generator, leaseManager), leaseManager.Close, nil
}

func buildInstanceID(httpAddress string) (string, error) {
	host, err := os.Hostname()
	if err != nil {
		return "", err
	}
	_, port, err := net.SplitHostPort(httpAddress)
	if err != nil {
		if len(httpAddress) > 0 && httpAddress[0] == ':' {
			port = httpAddress[1:]
		} else {
			return "", fmt.Errorf("parse http address %q: %w", httpAddress, err)
		}
	}
	return fmt.Sprintf("%s:%s", host, port), nil
}
