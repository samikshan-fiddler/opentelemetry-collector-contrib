package fiddlerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fiddlerreceiver"

import (
	"fmt"
	"time"
)

const (
	defaultEndpoint  = "localhost:8080"
	defaultAuthToken = ""
	defaultTimeout   = 5 * time.Minute
	defaultInterval  = 30 * time.Minute
	minimumInterval  = 5 * time.Minute
)

// Config defines configuration for the Fiddler receiver
type Config struct {
	// Endpoint for the Fiddler API (e.g., https://app.fiddler.ai)
	Endpoint string `mapstructure:"endpoint"`

	// Token is the Fiddler API key for authentication
	Token string `mapstructure:"token"`

	// TimeoutSettings configures the timeout settings for API calls
	Timeout time.Duration `mapstructure:"timeout"`

	// Interval for data collection (minimum 5 minutes)
	Interval time.Duration `mapstructure:"interval"`

	// EnabledMetrics is the list of metrics to collect
	EnabledMetrics []string `mapstructure:"enabled_metrics"`
}

func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("endpoint must be specified")
	}

	if cfg.Token == "" {
		return fmt.Errorf("token must be specified")
	}

	if cfg.Interval == 0 {
		cfg.Interval = defaultInterval
		return nil
	}

	if cfg.Interval < minimumInterval {
		return fmt.Errorf("interval must be at least 5 minutes")
	}

	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	return nil
}
