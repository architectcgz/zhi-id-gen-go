package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/architectcgz/zhi-id-gen-go/internal/platform/bootstrap"
	idgenruntime "github.com/architectcgz/zhi-id-gen-go/internal/services/idgen/runtime"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	if err := run(ctx); err != nil {
		log.Printf("zhi-id-gen-go exit with error: %v", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	app, err := bootstrap.New(ctx, "id-generator-service")
	if err != nil {
		return err
	}

	runtimeOptions, err := idgenruntime.Build(app)
	if err != nil {
		return err
	}

	if err := app.RegisterRuntime(runtimeOptions); err != nil {
		return err
	}

	log.Printf("zhi-id-gen-go listening on %s", app.Config.HTTPAddress)
	if err := app.Run(ctx); err != nil {
		return err
	}
	return nil
}
