// Package lifecycle coordinates the concurrent services that make up an
// application process.
package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrServiceStopped = errors.New("service stopped unexpectedly")

// Service describes one long-running process component. Run must block until
// the service stops. Shutdown may be nil when cancellation alone stops it.
type Service struct {
	Name     string
	Run      func(context.Context) error
	Shutdown func(context.Context) error
}

type serviceResult struct {
	name string
	err  error
}

// Run starts every service, cancels siblings when one stops or ctx is done,
// invokes shutdown hooks concurrently, and waits for all service goroutines.
func Run(ctx context.Context, shutdownTimeout time.Duration, services ...Service) error {
	if err := validate(ctx, shutdownTimeout, services); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	runResults := make(chan serviceResult, len(services))
	for _, service := range services {
		service := service
		go func() {
			runResults <- serviceResult{name: service.Name, err: service.Run(runCtx)}
		}()
	}

	completed := 0
	var runErr error
	select {
	case <-ctx.Done():
	case result := <-runResults:
		completed++
		if ctx.Err() == nil {
			if result.err == nil {
				runErr = fmt.Errorf("%s: %w", result.name, ErrServiceStopped)
			} else {
				runErr = fmt.Errorf("%s stopped: %w", result.name, result.err)
			}
		}
	}
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()

	shutdownResults := make(chan serviceResult, len(services))
	shutdownsPending := 0
	for _, service := range services {
		if service.Shutdown == nil {
			continue
		}
		shutdownsPending++
		service := service
		go func() {
			shutdownResults <- serviceResult{name: service.Name, err: service.Shutdown(shutdownCtx)}
		}()
	}

	for completed < len(services) || shutdownsPending > 0 {
		select {
		case result := <-runResults:
			completed++
			if result.err != nil && !isContextTermination(result.err) {
				runErr = errors.Join(runErr, fmt.Errorf("%s stopped: %w", result.name, result.err))
			}
		case result := <-shutdownResults:
			shutdownsPending--
			if result.err != nil {
				runErr = errors.Join(runErr, fmt.Errorf("shut down %s: %w", result.name, result.err))
			}
		case <-shutdownCtx.Done():
			return errors.Join(
				runErr,
				fmt.Errorf(
					"wait for services (%d running, %d shutting down): %w",
					len(services)-completed,
					shutdownsPending,
					shutdownCtx.Err(),
				),
			)
		}
	}

	return runErr
}

func validate(ctx context.Context, shutdownTimeout time.Duration, services []Service) error {
	if ctx == nil {
		return errors.New("lifecycle context is required")
	}
	if shutdownTimeout <= 0 {
		return errors.New("shutdown timeout must be greater than zero")
	}
	if len(services) == 0 {
		return errors.New("at least one service is required")
	}

	names := make(map[string]struct{}, len(services))
	for _, service := range services {
		if strings.TrimSpace(service.Name) == "" {
			return errors.New("service name is required")
		}
		if service.Run == nil {
			return fmt.Errorf("service %q has no Run function", service.Name)
		}
		if _, duplicate := names[service.Name]; duplicate {
			return fmt.Errorf("duplicate service name %q", service.Name)
		}
		names[service.Name] = struct{}{}
	}
	return nil
}

func isContextTermination(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
