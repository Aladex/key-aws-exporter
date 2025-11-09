package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"key-aws-exporter/internal/config"
	"key-aws-exporter/internal/exporter"
	"key-aws-exporter/internal/handlers"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()
	log.SetLevel(logrus.InfoLevel)
	log.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.WithError(err).Fatal("Failed to load configuration")
	}

	// Create validator manager for all endpoints
	manager := exporter.NewValidatorManager(cfg, log)

	log.WithFields(logrus.Fields{
		"port":            cfg.Port,
		"endpoints_count": manager.GetEndpointCount(),
	}).Info("Starting AWS S3 Key Exporter")

	for _, endpoint := range manager.GetEndpoints() {
		log.WithField("endpoint", endpoint).Debug("Configured S3 endpoint")
	}

	// Create HTTP mux
	mux := http.NewServeMux()

	// Register Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Register health check endpoint
	mux.HandleFunc("/health", handlers.NewHealthCheckHandler(manager))

	// Register validation endpoints
	// POST /validate - validate all endpoints
	mux.HandleFunc("/validate", handlers.NewValidateAllHandler(manager, log))

	// POST/GET /validate/{endpoint} - validate specific endpoint
	mux.HandleFunc("/validate/", handlers.NewValidateEndpointHandler(manager, log))

	// Start server
	addr := fmt.Sprintf(":%d", cfg.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.WithField("addr", addr).Info("HTTP server listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("Server error")
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Info("Shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.WithError(err).Error("Server shutdown error")
	}
}
