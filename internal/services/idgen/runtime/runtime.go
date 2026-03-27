package runtime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/architectcgz/zhi-id-gen-go/internal/platform/bootstrap"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/commands"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/app/queries"
	"github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/domain"
	segmentpostgres "github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/infra/postgres"
	transporthttp "github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/transport/http"
)

func Build(app *bootstrap.App) (bootstrap.RuntimeOptions, error) {
	if app.DB == nil {
		return bootstrap.RuntimeOptions{}, errors.New("DATABASE_URL is required for id-generator runtime")
	}

	segmentRepository := segmentpostgres.NewSegmentRepository(app.DB)
	segmentAllocator := commands.NewCachedSegmentAllocator(segmentRepository, nil)
	snowflakeService, closeFn, err := buildSnowflakeService(app)
	if err != nil {
		return bootstrap.RuntimeOptions{}, err
	}
	healthQuery := queries.NewHealthQueryService(app.Config.ServiceName, segmentRepository, snowflakeService)
	cacheQuery := queries.NewSegmentCacheQueryService(segmentAllocator, segmentRepository.IsInitialized)
	handler := transporthttp.NewHandler(
		healthQuery,
		commands.NewSegmentCommandService(segmentAllocator),
		queries.NewTagsQueryService(segmentRepository),
		snowflakeService,
		snowflakeService,
		cacheQuery,
	)
	return bootstrap.RuntimeOptions{
		Handler: handler.Routes(),
		Close:   closeFn,
	}, nil
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
