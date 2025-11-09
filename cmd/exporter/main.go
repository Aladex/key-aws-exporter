package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"key-aws-exporter/internal/config"
	"key-aws-exporter/internal/exporter"
	"key-aws-exporter/internal/handlers"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

type serverRunner interface {
	ListenAndServe() error
	Shutdown(context.Context) error
}

type validationRunner interface {
	ValidateAll(ctx context.Context) *exporter.ValidationResults
}

func main() {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	server, manager := createServer(cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	startAutoValidation(ctx, manager, log, cfg.AutoValidateInterval)

	if err := runServer(ctx, server, server.Addr, log); err != nil {
		log.WithError(err).Fatal("Server error")
	}
}

func createServer(cfg *config.Config, log *logrus.Logger) (*http.Server, *exporter.ValidatorManager) {
	manager := exporter.NewValidatorManager(cfg, log)

	log.WithFields(logrus.Fields{
		"port":            cfg.Port,
		"endpoints_count": manager.GetEndpointCount(),
	}).Info("Starting AWS S3 Key Exporter")

	for _, endpoint := range manager.GetEndpoints() {
		log.WithField("endpoint", endpoint).Debug("Configured S3 endpoint")
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", handlers.NewHealthCheckHandler(manager))
	mux.HandleFunc("/validate", handlers.NewValidateAllHandler(manager, log))
	mux.HandleFunc("/validate/", handlers.NewValidateEndpointHandler(manager, log))

	addr := fmt.Sprintf(":%d", cfg.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return server, manager
}

func runServer(ctx context.Context, server serverRunner, addr string, log *logrus.Logger) error {
	errCh := make(chan error, 1)

	go func() {
		log.WithField("addr", addr).Info("HTTP server listening")
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		log.Info("Shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}

		err := <-errCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return err
	}
}

func startAutoValidation(ctx context.Context, manager validationRunner, log *logrus.Logger, interval time.Duration) {
	if interval <= 0 {
		return
	}

	go func() {
		runValidation := func() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			results := manager.ValidateAll(ctx)
			for endpoint, result := range results.Results {
				exporter.RecordResult(log, endpoint, result)
			}
		}

		runValidation()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runValidation()
			}
		}
	}()
}
