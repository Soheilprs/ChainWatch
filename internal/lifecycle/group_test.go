package lifecycle

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunStopsEveryServiceAfterCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var shutdowns atomic.Int32
	service := func(name string) Service {
		return Service{
			Name: name,
			Run: func(ctx context.Context) error {
				<-ctx.Done()
				return ctx.Err()
			},
			Shutdown: func(context.Context) error {
				shutdowns.Add(1)
				return nil
			},
		}
	}
	cancel()

	err := Run(ctx, time.Second, service("indexer"), service("HTTP server"))

	if err != nil {
		t.Fatalf("run canceled services: %v", err)
	}
	if got := shutdowns.Load(); got != 2 {
		t.Fatalf("shutdown calls = %d, want 2", got)
	}
}

func TestRunPropagatesFailureAndCancelsSiblings(t *testing.T) {
	wantErr := errors.New("listen failed")
	siblingCanceled := make(chan struct{})

	err := Run(
		context.Background(),
		time.Second,
		Service{Name: "HTTP server", Run: func(context.Context) error { return wantErr }},
		Service{Name: "indexer", Run: func(ctx context.Context) error {
			<-ctx.Done()
			close(siblingCanceled)
			return ctx.Err()
		}},
	)

	if !errors.Is(err, wantErr) || !strings.Contains(err.Error(), "HTTP server") {
		t.Fatalf("error = %v, want named service failure", err)
	}
	select {
	case <-siblingCanceled:
	default:
		t.Fatal("sibling service was not canceled")
	}
}

func TestRunRejectsUnexpectedCleanStop(t *testing.T) {
	err := Run(
		context.Background(),
		time.Second,
		Service{Name: "worker", Run: func(context.Context) error { return nil }},
	)

	if !errors.Is(err, ErrServiceStopped) {
		t.Fatalf("error = %v, want ErrServiceStopped", err)
	}
}

func TestRunPreservesShutdownFailures(t *testing.T) {
	shutdownErr := errors.New("drain failed")
	err := Run(
		context.Background(),
		time.Second,
		Service{
			Name: "server",
			Run:  func(context.Context) error { return errors.New("serve failed") },
			Shutdown: func(context.Context) error {
				return shutdownErr
			},
		},
	)

	if !errors.Is(err, shutdownErr) {
		t.Fatalf("error = %v, want shutdown failure", err)
	}
}

func TestRunTimesOutWaitingForService(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(
		ctx,
		10*time.Millisecond,
		Service{Name: "stuck", Run: func(context.Context) error {
			<-release
			return nil
		}},
	)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want shutdown deadline", err)
	}
}

func TestRunValidatesServices(t *testing.T) {
	tests := []struct {
		name     string
		services []Service
		want     string
	}{
		{name: "none", want: "at least one"},
		{name: "missing name", services: []Service{{Run: func(context.Context) error { return nil }}}, want: "name"},
		{name: "missing run", services: []Service{{Name: "worker"}}, want: "Run"},
		{
			name: "duplicate",
			services: []Service{
				{Name: "worker", Run: func(context.Context) error { return nil }},
				{Name: "worker", Run: func(context.Context) error { return nil }},
			},
			want: "duplicate",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Run(context.Background(), time.Second, test.services...)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want error containing %q", err, test.want)
			}
		})
	}
}
