package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/celsolenonsander/visionflow-api/internal/config"
	"github.com/celsolenonsander/visionflow-api/internal/db"
	"github.com/celsolenonsander/visionflow-api/internal/features/relatorio"
	"github.com/celsolenonsander/visionflow-api/internal/httpapi"
	"github.com/celsolenonsander/visionflow-api/internal/logger"
	"github.com/celsolenonsander/visionflow-api/internal/service"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("erro fatal: %v", err)
	}
}

func run() error {
	// 1. Config
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// 2. Contexto com cancelamento (graceful shutdown)
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// 3. Logger
	logg, err := logger.New()
	if err != nil {
		return err
	}
	defer logg.Close()

	// 4. Pool do banco
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	logg.Printf("✓ conectado em %s:%s/%s",
		cfg.DBHost, cfg.DBPort, cfg.DBName)

	if err := relatorio.InitializeSchemaCache(ctx, pool, logg); err != nil {
		logg.Printf("não foi possível preparar o cache de schema dos relatórios com IA: %v", err)
	}

	// 5. Service e Handler
	//    Service e handlers compartilham o pool PostgreSQL.
	svc := service.New(pool, logg)
	handler := httpapi.NewHandler(svc, pool, cfg.JWTSecret, cfg.CORSAllowedOrigins, logg)
	handler.AppEnv = cfg.AppEnv
	handler.SQLConsoleEnabled = cfg.SQLConsoleEnabled

	// 6. Router
	router := handler.Routes()

	// 7. Server HTTP
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logg.Printf("✓ servidor ouvindo em :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logg.Printf("→ encerrando servidor...")
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), 10*time.Second,
		)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
