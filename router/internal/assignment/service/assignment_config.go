package service

import (
	"fmt"
	"net/url"
	"time"

	"autoscale-distr-storage/router/internal/utils"
)

type AssignmentConfig struct {
	ControlPlaneURL string         `yaml:"control_plane_url"`
	RefreshInterval utils.Duration `yaml:"refresh_interval"`
	RequestTimeout  utils.Duration `yaml:"request_timeout"`
}

// Validate проверяет источник и интервалы обновления assignment.
func (assignmentConfig *AssignmentConfig) Validate() error {
	parsedURL, err := url.Parse(assignmentConfig.ControlPlaneURL)
	if err != nil || parsedURL.Scheme != "http" || parsedURL.Host == "" || parsedURL.Path != "" || parsedURL.RawQuery != "" || parsedURL.Fragment != "" || parsedURL.User != nil {
		return fmt.Errorf("control plane URL must be an HTTP origin")
	}
	if assignmentConfig.RefreshInterval.Time() < time.Millisecond || assignmentConfig.RequestTimeout.Time() < time.Millisecond {
		return fmt.Errorf("assignment refresh interval and timeout must be positive")
	}
	return nil
}
