package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"key-aws-exporter/internal/config"

	"github.com/sirupsen/logrus"
)

func TestCreateServerRegistersHandlers(t *testing.T) {
	cfg := &config.Config{
		Port:              9090,
		ValidationTimeout: time.Second,
		Endpoints: []config.S3EndpointConfig{
			{Name: "bucket", Bucket: "bucket", AccessKey: "ak", SecretKey: "sk"},
		},
	}

	server, manager := createServer(cfg, logrus.New())

	if manager.GetEndpointCount() != 1 {
		t.Fatalf("expected 1 endpoint, got %d", manager.GetEndpointCount())
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	server.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected health endpoint to return 200, got %d", rr.Code)
	}
}

type stubHTTPServer struct {
	listenBlock       chan struct{}
	returnImmediately bool
	listenErr         error
	shutdownErr       error
	listenCalled      bool
	shutdownCalled    bool
}

func newStubHTTPServer() *stubHTTPServer {
	return &stubHTTPServer{listenBlock: make(chan struct{})}
}

func (s *stubHTTPServer) ListenAndServe() error {
	s.listenCalled = true
	if s.returnImmediately {
		if s.listenErr != nil {
			return s.listenErr
		}
		return nil
	}
	<-s.listenBlock
	if s.listenErr != nil {
		return s.listenErr
	}
	return http.ErrServerClosed
}

func (s *stubHTTPServer) Shutdown(ctx context.Context) error {
	s.shutdownCalled = true
	close(s.listenBlock)
	return s.shutdownErr
}

func TestRunServerShutsDownOnContext(t *testing.T) {
	stub := newStubHTTPServer()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- runServer(ctx, stub, ":0", logrus.New())
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	if err := <-done; err != nil {
		t.Fatalf("runServer returned error: %v", err)
	}

	if !stub.listenCalled {
		t.Fatalf("expected ListenAndServe to be called")
	}
	if !stub.shutdownCalled {
		t.Fatalf("expected Shutdown to be called")
	}
}

func TestRunServerPropagatesErrors(t *testing.T) {
	stub := newStubHTTPServer()
	stub.returnImmediately = true
	stub.listenErr = errors.New("boom")

	err := runServer(context.Background(), stub, ":0", logrus.New())
	if err == nil || !errors.Is(err, stub.listenErr) {
		t.Fatalf("expected listen error, got %v", err)
	}
}
