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
	if config.ProfilingAddress != "" {
		t.Fatalf("profiling address = %q, want profiling disabled", config.ProfilingAddress)
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
	if config.ReadTimeout != defaultReadTimeout ||
		config.WriteTimeout != defaultWriteTimeout ||
		config.IdleTimeout != defaultIdleTimeout {
		t.Fatalf("unexpected HTTP timeout defaults: %+v", config)
	}
	if config.ConfirmationDepth != defaultConfirmationDepth {
		t.Fatalf("confirmation depth = %d, want %d", config.ConfirmationDepth, defaultConfirmationDepth)
	}
	if config.MaxReorgDepth != defaultMaxReorgDepth {
		t.Fatalf("max reorg depth = %d, want %d", config.MaxReorgDepth, defaultMaxReorgDepth)
	}
	if config.RPCMaxAttempts != defaultRPCMaxAttempts ||
		config.RPCInitialBackoff != defaultRPCInitialBackoff ||
		config.RPCMaxBackoff != defaultRPCMaxBackoff ||
		config.RPCRequestsPerSecond != defaultRPCRateLimit ||
		config.RPCBurst != defaultRPCBurst {
		t.Fatalf("unexpected RPC resilience defaults: %+v", config.RPCResilienceConfig())
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
		"ETH_RPC_URL":             "http://localhost:8545",
		"DATABASE_URL":            "postgres://localhost/chainwatch",
		"HTTP_ADDRESS":            "127.0.0.1:9090",
		"PPROF_ADDRESS":           "127.0.0.1:6060",
		"WORKER_COUNT":            "8",
		"POLL_INTERVAL":           "750ms",
		"SHUTDOWN_TIMEOUT":        "12s",
		"READ_HEADER_TIMEOUT":     "3s",
		"READ_TIMEOUT":            "10s",
		"WRITE_TIMEOUT":           "20s",
		"IDLE_TIMEOUT":            "45s",
		"CONFIRMATION_DEPTH":      "18",
		"MAX_REORG_DEPTH":         "96",
		"RPC_MAX_ATTEMPTS":        "6",
		"RPC_INITIAL_BACKOFF":     "125ms",
		"RPC_MAX_BACKOFF":         "4s",
		"RPC_REQUESTS_PER_SECOND": "40",
		"RPC_BURST":               "16",
	}))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if config.HTTPAddress != "127.0.0.1:9090" ||
		config.ProfilingAddress != "127.0.0.1:6060" ||
		config.WorkerCount != 8 ||
		config.PollInterval != 750*time.Millisecond ||
		config.ShutdownTimeout != 12*time.Second ||
		config.ReadHeaderTimeout != 3*time.Second ||
		config.ReadTimeout != 10*time.Second ||
		config.WriteTimeout != 20*time.Second ||
		config.IdleTimeout != 45*time.Second ||
		config.ConfirmationDepth != 18 ||
		config.MaxReorgDepth != 96 ||
		config.RPCMaxAttempts != 6 ||
		config.RPCInitialBackoff != 125*time.Millisecond ||
		config.RPCMaxBackoff != 4*time.Second ||
		config.RPCRequestsPerSecond != 40 ||
		config.RPCBurst != 16 {
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
		{name: "pprof public bind", key: "PPROF_ADDRESS", value: "0.0.0.0:6060", want: "loopback"},
		{name: "pprof missing port", key: "PPROF_ADDRESS", value: "localhost", want: "missing port"},
		{name: "worker zero", key: "WORKER_COUNT", value: "0", want: "WORKER_COUNT must be greater than zero"},
		{name: "poll syntax", key: "POLL_INTERVAL", value: "soon", want: "parse POLL_INTERVAL"},
		{name: "poll zero", key: "POLL_INTERVAL", value: "0s", want: "POLL_INTERVAL must be greater than zero"},
		{name: "shutdown negative", key: "SHUTDOWN_TIMEOUT", value: "-1s", want: "SHUTDOWN_TIMEOUT must be greater than zero"},
		{name: "read header zero", key: "READ_HEADER_TIMEOUT", value: "0s", want: "READ_HEADER_TIMEOUT must be greater than zero"},
		{name: "read zero", key: "READ_TIMEOUT", value: "0s", want: "READ_TIMEOUT must be greater than zero"},
		{name: "write zero", key: "WRITE_TIMEOUT", value: "0s", want: "WRITE_TIMEOUT must be greater than zero"},
		{name: "idle zero", key: "IDLE_TIMEOUT", value: "0s", want: "IDLE_TIMEOUT must be greater than zero"},
		{name: "confirmation syntax", key: "CONFIRMATION_DEPTH", value: "many", want: "parse CONFIRMATION_DEPTH"},
		{name: "max reorg zero", key: "MAX_REORG_DEPTH", value: "0", want: "MAX_REORG_DEPTH must be greater than zero"},
		{name: "RPC attempts zero", key: "RPC_MAX_ATTEMPTS", value: "0", want: "RPC max attempts must be greater than zero"},
		{name: "RPC initial backoff syntax", key: "RPC_INITIAL_BACKOFF", value: "later", want: "parse RPC_INITIAL_BACKOFF"},
		{name: "RPC backoff ordering", key: "RPC_MAX_BACKOFF", value: "100ms", want: "RPC max backoff must be at least"},
		{name: "RPC rate zero", key: "RPC_REQUESTS_PER_SECOND", value: "0", want: "RPC requests per second must be greater than zero"},
		{name: "RPC burst zero", key: "RPC_BURST", value: "0", want: "RPC burst must be greater than zero"},
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
