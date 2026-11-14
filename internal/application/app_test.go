package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

const testTimeout = time.Second

type fakeServer struct {
	started     chan struct{}
	stopped     chan struct{}
	release     <-chan struct{}
	startErr    error
	shutdownErr error
	startOnce   sync.Once
	stopOnce    sync.Once
}

func newFakeServer() *fakeServer {
	return &fakeServer{
		started: make(chan struct{}),
		stopped: make(chan struct{}),
	}
}

func (s *fakeServer) Start() error {
	s.startOnce.Do(func() { close(s.started) })
	if s.release != nil {
		<-s.release
	}
	if s.startErr != nil {
		return s.startErr
	}
	<-s.stopped
	return nil
}

func (s *fakeServer) Shutdown(context.Context) error {
	s.stopOnce.Do(func() { close(s.stopped) })
	return s.shutdownErr
}

func TestRunStartsAndShutsDownEveryServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	servers := []*fakeServer{newFakeServer(), newFakeServer()}
	done := make(chan error, 1)

	go func() {
		done <- run(ctx, servers[0], servers[1])
	}()

	for _, server := range servers {
		waitFor(t, server.started)
	}
	cancel()

	if err := waitFor(t, done); err != nil {
		t.Fatalf("run() error = %v, want nil", err)
	}
	for _, server := range servers {
		waitFor(t, server.stopped)
	}
}

func TestRunShutsDownEveryServerWhenOneFails(t *testing.T) {
	startErr := errors.New("start failed")
	release := make(chan struct{})
	failingServer := newFakeServer()
	failingServer.release = release
	failingServer.startErr = startErr
	otherServer := newFakeServer()
	otherServer.release = release
	done := make(chan error, 1)

	go func() {
		done <- run(context.Background(), failingServer, otherServer)
	}()

	waitFor(t, failingServer.started)
	waitFor(t, otherServer.started)
	close(release)

	if err := waitFor(t, done); !errors.Is(err, startErr) {
		t.Fatalf("run() error = %v, want %v", err, startErr)
	}
	waitFor(t, failingServer.stopped)
	waitFor(t, otherServer.stopped)
}

func TestRunReturnsShutdownErrors(t *testing.T) {
	shutdownErr := errors.New("shutdown failed")
	server := newFakeServer()
	server.shutdownErr = shutdownErr
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)

	go func() {
		done <- run(ctx, server)
	}()

	waitFor(t, server.started)
	cancel()

	if err := waitFor(t, done); !errors.Is(err, shutdownErr) {
		t.Fatalf("run() error = %v, want %v", err, shutdownErr)
	}
}

func waitFor[T any](t *testing.T, channel <-chan T) T {
	t.Helper()
	select {
	case value := <-channel:
		return value
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for lifecycle event")
		var zero T
		return zero
	}
}
