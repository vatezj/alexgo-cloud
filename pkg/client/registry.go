package client

import (
	"fmt"

	"alexGo-cloud/pkg/config"
)

func GetServiceAddress(cfg *config.Config, serviceName string) (string, error) {
	if cfg == nil || !cfg.Microservice.Enabled {
		return "", fmt.Errorf("microservice mode not enabled")
	}
	_ = serviceName
	return "localhost:50051", nil
}
