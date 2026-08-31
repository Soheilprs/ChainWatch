package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddress       = ":8080"
	defaultWorkerCount       = 3
	defaultPollInterval      = 4 * time.Second
	defaultShutdownTimeout   = 5 * time.Second
	defaultReadHeaderTimeout = 5 * time.Second
	defaultReadTimeout       = 15 * time.Second
	defaultWriteTimeout      = 30 * time.Second
	defaultIdleTimeout       = 60 * time.Second
	defaultConfirmationDepth = uint64(12)
	defaultMaxReorgDepth     = uint64(64)
	defaultRPCMaxAttempts    = 4
	defaultRPCInitialBackoff = 200 * time.Millisecond
	defaultRPCMaxBackoff     = 3 * time.Second
	defaultRPCRateLimit      = 20
	defaultRPCBurst          = 10
)

// Config contains all runtime settings needed to start ChainWatch.
type Config struct {
	EthereumRPCURL       string
	DatabaseURL          string
	HTTPAddress          string
	WorkerCount          int
	PollInterval         time.Duration
	ShutdownTimeout      time.Duration
	ReadHeaderTimeout    time.Duration
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	IdleTimeout          time.Duration
	ConfirmationDepth    uint64
	MaxReorgDepth        uint64
	RPCMaxAttempts       int
	RPCInitialBackoff    time.Duration
	RPCMaxBackoff        time.Duration
	RPCRequestsPerSecond int
	RPCBurst             int
}

// LoadConfig reads and validates ChainWatch configuration from the process
// environment.
func LoadConfig() (Config, error) {
	config, err := loadConfig(os.LookupEnv)
	if err != nil {
		return Config{}, NewDomainError(
			ErrConfiguration,
			"load environment configuration",
			err,
		)
	}
	return config, nil
}

type environmentLookup func(string) (string, bool)

func loadConfig(lookup environmentLookup) (Config, error) {
	if lookup == nil {
		return Config{}, errors.New("environment lookup is required")
	}

	config := Config{
		HTTPAddress:          defaultHTTPAddress,
		WorkerCount:          defaultWorkerCount,
		PollInterval:         defaultPollInterval,
		ShutdownTimeout:      defaultShutdownTimeout,
		ReadHeaderTimeout:    defaultReadHeaderTimeout,
		ReadTimeout:          defaultReadTimeout,
		WriteTimeout:         defaultWriteTimeout,
		IdleTimeout:          defaultIdleTimeout,
		ConfirmationDepth:    defaultConfirmationDepth,
		MaxReorgDepth:        defaultMaxReorgDepth,
		RPCMaxAttempts:       defaultRPCMaxAttempts,
		RPCInitialBackoff:    defaultRPCInitialBackoff,
		RPCMaxBackoff:        defaultRPCMaxBackoff,
		RPCRequestsPerSecond: defaultRPCRateLimit,
		RPCBurst:             defaultRPCBurst,
	}

	var err error
	if config.EthereumRPCURL, err = requiredEnvironment(lookup, "ETH_RPC_URL"); err != nil {
		return Config{}, err
	}
	if config.DatabaseURL, err = requiredEnvironment(lookup, "DATABASE_URL"); err != nil {
		return Config{}, err
	}

	if value, exists := optionalEnvironment(lookup, "HTTP_ADDRESS"); exists {
		config.HTTPAddress = value
	}
	if value, exists := optionalEnvironment(lookup, "WORKER_COUNT"); exists {
		config.WorkerCount, err = strconv.Atoi(value)
		if err != nil {
			return Config{}, NewDomainError(ErrValidation, "parse WORKER_COUNT", err)
		}
	}
	if config.PollInterval, err = durationEnvironment(lookup, "POLL_INTERVAL", config.PollInterval); err != nil {
		return Config{}, err
	}
	if config.ShutdownTimeout, err = durationEnvironment(lookup, "SHUTDOWN_TIMEOUT", config.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if config.ReadHeaderTimeout, err = durationEnvironment(lookup, "READ_HEADER_TIMEOUT", config.ReadHeaderTimeout); err != nil {
		return Config{}, err
	}
	if config.ReadTimeout, err = durationEnvironment(lookup, "READ_TIMEOUT", config.ReadTimeout); err != nil {
		return Config{}, err
	}
	if config.WriteTimeout, err = durationEnvironment(lookup, "WRITE_TIMEOUT", config.WriteTimeout); err != nil {
		return Config{}, err
	}
	if config.IdleTimeout, err = durationEnvironment(lookup, "IDLE_TIMEOUT", config.IdleTimeout); err != nil {
		return Config{}, err
	}
	if config.ConfirmationDepth, err = uint64Environment(lookup, "CONFIRMATION_DEPTH", config.ConfirmationDepth); err != nil {
		return Config{}, err
	}
	if config.MaxReorgDepth, err = uint64Environment(lookup, "MAX_REORG_DEPTH", config.MaxReorgDepth); err != nil {
		return Config{}, err
	}
	if config.RPCMaxAttempts, err = intEnvironment(lookup, "RPC_MAX_ATTEMPTS", config.RPCMaxAttempts); err != nil {
		return Config{}, err
	}
	if config.RPCInitialBackoff, err = durationEnvironment(lookup, "RPC_INITIAL_BACKOFF", config.RPCInitialBackoff); err != nil {
		return Config{}, err
	}
	if config.RPCMaxBackoff, err = durationEnvironment(lookup, "RPC_MAX_BACKOFF", config.RPCMaxBackoff); err != nil {
		return Config{}, err
	}
	if config.RPCRequestsPerSecond, err = intEnvironment(lookup, "RPC_REQUESTS_PER_SECOND", config.RPCRequestsPerSecond); err != nil {
		return Config{}, err
	}
	if config.RPCBurst, err = intEnvironment(lookup, "RPC_BURST", config.RPCBurst); err != nil {
		return Config{}, err
	}

	if err := config.Validate(); err != nil {
		return Config{}, err
	}

	return config, nil
}

func (c Config) Validate() error {
	switch {
	case strings.TrimSpace(c.EthereumRPCURL) == "":
		return NewDomainError(ErrValidation, "validate configuration", errors.New("ETH_RPC_URL is required"))
	case strings.TrimSpace(c.DatabaseURL) == "":
		return NewDomainError(ErrValidation, "validate configuration", errors.New("DATABASE_URL is required"))
	case strings.TrimSpace(c.HTTPAddress) == "":
		return NewDomainError(ErrValidation, "validate configuration", errors.New("HTTP_ADDRESS must not be empty"))
	case c.WorkerCount <= 0:
		return NewDomainError(ErrValidation, "validate configuration", errors.New("WORKER_COUNT must be greater than zero"))
	case c.PollInterval <= 0:
		return NewDomainError(ErrValidation, "validate configuration", errors.New("POLL_INTERVAL must be greater than zero"))
	case c.ShutdownTimeout <= 0:
		return NewDomainError(ErrValidation, "validate configuration", errors.New("SHUTDOWN_TIMEOUT must be greater than zero"))
	case c.ReadHeaderTimeout <= 0:
		return NewDomainError(ErrValidation, "validate configuration", errors.New("READ_HEADER_TIMEOUT must be greater than zero"))
	case c.ReadTimeout <= 0:
		return NewDomainError(ErrValidation, "validate configuration", errors.New("READ_TIMEOUT must be greater than zero"))
	case c.WriteTimeout <= 0:
		return NewDomainError(ErrValidation, "validate configuration", errors.New("WRITE_TIMEOUT must be greater than zero"))
	case c.IdleTimeout <= 0:
		return NewDomainError(ErrValidation, "validate configuration", errors.New("IDLE_TIMEOUT must be greater than zero"))
	case c.MaxReorgDepth == 0:
		return NewDomainError(ErrValidation, "validate configuration", errors.New("MAX_REORG_DEPTH must be greater than zero"))
	default:
		if err := c.RPCResilienceConfig().Validate(); err != nil {
			return NewDomainError(
				ErrValidation,
				"validate RPC resilience configuration",
				err,
			)
		}
		return nil
	}
}

func (c Config) RPCResilienceConfig() RPCResilienceConfig {
	return RPCResilienceConfig{
		MaxAttempts:       c.RPCMaxAttempts,
		InitialBackoff:    c.RPCInitialBackoff,
		MaxBackoff:        c.RPCMaxBackoff,
		JitterFraction:    0.2,
		RequestsPerSecond: c.RPCRequestsPerSecond,
		Burst:             c.RPCBurst,
	}
}

func intEnvironment(
	lookup environmentLookup,
	name string,
	defaultValue int,
) (int, error) {
	value, exists := optionalEnvironment(lookup, name)
	if !exists {
		return defaultValue, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, NewDomainError(ErrValidation, "parse "+name, err)
	}
	return parsed, nil
}

func uint64Environment(
	lookup environmentLookup,
	name string,
	defaultValue uint64,
) (uint64, error) {
	value, exists := optionalEnvironment(lookup, name)
	if !exists {
		return defaultValue, nil
	}

	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, NewDomainError(ErrValidation, "parse "+name, err)
	}
	return parsed, nil
}

func requiredEnvironment(lookup environmentLookup, name string) (string, error) {
	value, exists := optionalEnvironment(lookup, name)
	if !exists {
		return "", NewDomainError(
			ErrValidation,
			"validate configuration",
			fmt.Errorf("%s is required", name),
		)
	}
	return value, nil
}

func optionalEnvironment(lookup environmentLookup, name string) (string, bool) {
	value, exists := lookup(name)
	value = strings.TrimSpace(value)
	return value, exists && value != ""
}

func durationEnvironment(
	lookup environmentLookup,
	name string,
	defaultValue time.Duration,
) (time.Duration, error) {
	value, exists := optionalEnvironment(lookup, name)
	if !exists {
		return defaultValue, nil
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, NewDomainError(ErrValidation, "parse "+name, err)
	}
	return duration, nil
}
