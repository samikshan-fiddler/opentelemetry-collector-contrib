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

var defaultEnabledMetricTypes = []string{"drift", "traffic", "performance", "statistic", "service_metrics"}

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

	// EnabledMetricTypes is the list of metric types to collect
	EnabledMetricTypes []string `mapstructure:"enabled_metric_types"`
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
		return fmt.Errorf("interval must be at least %d minutes", minimumInterval/time.Minute)
	}

	if cfg.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than 0")
	}

	return nil
}
