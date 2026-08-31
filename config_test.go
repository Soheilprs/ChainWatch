package main

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigUsesDefaults(t *testing.T) {
	config, err := loadConfig(mapEnvironment(map[string]string{
		"ETH_RPC_URL":  "https://ethereum.example",
		"DATABASE_URL": "postgres://chainwatch@example/chainwatch",
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if config.EthereumRPCURL != "https://ethereum.example" {
		t.Fatalf("unexpected RPC URL: %q", config.EthereumRPCURL)
	}
	if config.DatabaseURL != "postgres://chainwatch@example/chainwatch" {
		t.Fatalf("unexpected database URL: %q", config.DatabaseURL)
	}
	if config.HTTPAddress != defaultHTTPAddress {
		t.Fatalf("HTTP address = %q, want %q", config.HTTPAddress, defaultHTTPAddress)
	}
	if config.WorkerCount != defaultWorkerCount {
		t.Fatalf("worker count = %d, want %d", config.WorkerCount, defaultWorkerCount)
	}
	if config.PollInterval != defaultPollInterval {
		t.Fatalf("poll interval = %s, want %s", config.PollInterval, defaultPollInterval)
	}
	if config.ShutdownTimeout != defaultShutdownTimeout {
		t.Fatalf("shutdown timeout = %s, want %s", config.ShutdownTimeout, defaultShutdownTimeout)
	}
	if config.ReadHeaderTimeout != defaultReadHeaderTimeout {
		t.Fatalf("read header timeout = %s, want %s", config.ReadHeaderTimeout, defaultReadHeaderTimeout)
	}
}

func TestLoadConfigClassifiesConfigurationValidation(t *testing.T) {
	t.Setenv("ETH_RPC_URL", "")
	t.Setenv("DATABASE_URL", "postgres://localhost/chainwatch")

	_, err := LoadConfig()
	if !errors.Is(err, ErrConfiguration) {
		t.Fatalf("expected configuration error, got %v", err)
	}
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestLoadConfigReadsOverrides(t *testing.T) {
	config, err := loadConfig(mapEnvironment(map[string]string{
		"ETH_RPC_URL":         "http://localhost:8545",
		"DATABASE_URL":        "postgres://localhost/chainwatch",
		"HTTP_ADDRESS":        "127.0.0.1:9090",
		"WORKER_COUNT":        "8",
		"POLL_INTERVAL":       "750ms",
		"SHUTDOWN_TIMEOUT":    "12s",
		"READ_HEADER_TIMEOUT": "3s",
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if config.HTTPAddress != "127.0.0.1:9090" ||
		config.WorkerCount != 8 ||
		config.PollInterval != 750*time.Millisecond ||
		config.ShutdownTimeout != 12*time.Second ||
		config.ReadHeaderTimeout != 3*time.Second {
		t.Fatalf("unexpected overrides: %+v", config)
	}
}

func TestLoadConfigRejectsMissingRequiredValues(t *testing.T) {
	tests := []struct {
		name        string
		environment map[string]string
		want        string
	}{
		{name: "RPC URL", environment: map[string]string{"DATABASE_URL": "postgres://db"}, want: "ETH_RPC_URL is required"},
		{name: "database URL", environment: map[string]string{"ETH_RPC_URL": "http://rpc"}, want: "DATABASE_URL is required"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := loadConfig(mapEnvironment(test.environment))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want error containing %q", err, test.want)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
		want  string
	}{
		{name: "worker syntax", key: "WORKER_COUNT", value: "many", want: "parse WORKER_COUNT"},
		{name: "worker zero", key: "WORKER_COUNT", value: "0", want: "WORKER_COUNT must be greater than zero"},
		{name: "poll syntax", key: "POLL_INTERVAL", value: "soon", want: "parse POLL_INTERVAL"},
		{name: "poll zero", key: "POLL_INTERVAL", value: "0s", want: "POLL_INTERVAL must be greater than zero"},
		{name: "shutdown negative", key: "SHUTDOWN_TIMEOUT", value: "-1s", want: "SHUTDOWN_TIMEOUT must be greater than zero"},
		{name: "read header zero", key: "READ_HEADER_TIMEOUT", value: "0s", want: "READ_HEADER_TIMEOUT must be greater than zero"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			environment := map[string]string{
				"ETH_RPC_URL":  "http://rpc",
				"DATABASE_URL": "postgres://db",
				test.key:       test.value,
			}
			_, err := loadConfig(mapEnvironment(environment))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want error containing %q", err, test.want)
			}
		})
	}
}

func mapEnvironment(values map[string]string) environmentLookup {
	return func(name string) (string, bool) {
		value, exists := values[name]
		return value, exists
	}
}
