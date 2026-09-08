package config

import (
	"errors"
	"os"
	"strings"
)

type Config struct {
	ListenAddress          string
	GatewayListenAddress   string
	GatewayCertificateFile string
	GatewayPrivateKeyFile  string
	GatewayCAFile          string
	DatabaseURL            string
	Environment            string
}

func Load(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}
	rawListenAddress := getenv("API_LISTEN_ADDRESS")
	rawDatabaseURL := getenv("DATABASE_URL")
	rawEnvironment := getenv("APP_ENV")
	rawGatewayAddress := getenv("GATEWAY_LISTEN_ADDRESS")
	rawGatewayCert := getenv("GATEWAY_TLS_CERT_FILE")
	rawGatewayKey := getenv("GATEWAY_TLS_KEY_FILE")
	rawGatewayCA := getenv("GATEWAY_TLS_CA_FILE")
	if strings.ContainsAny(rawDatabaseURL+rawListenAddress+rawEnvironment+rawGatewayAddress+rawGatewayCert+rawGatewayKey+rawGatewayCA, "\r\n") {
		return Config{}, errors.New("runtime configuration contains newline")
	}
	config := Config{
		ListenAddress:          strings.TrimSpace(rawListenAddress),
		GatewayListenAddress:   strings.TrimSpace(rawGatewayAddress),
		GatewayCertificateFile: strings.TrimSpace(rawGatewayCert),
		GatewayPrivateKeyFile:  strings.TrimSpace(rawGatewayKey),
		GatewayCAFile:          strings.TrimSpace(rawGatewayCA),
		DatabaseURL:            strings.TrimSpace(rawDatabaseURL),
		Environment:            strings.TrimSpace(rawEnvironment),
	}
	if config.ListenAddress == "" {
		config.ListenAddress = ":8080"
	}
	if config.Environment == "" {
		config.Environment = "development"
	}
	if config.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	configuredTLS := []string{config.GatewayCertificateFile, config.GatewayPrivateKeyFile, config.GatewayCAFile}
	if (configuredTLS[0] != "" || configuredTLS[1] != "" || configuredTLS[2] != "") && (configuredTLS[0] == "" || configuredTLS[1] == "" || configuredTLS[2] == "") {
		return Config{}, errors.New("gateway TLS requires certificate, key, and CA files")
	}
	if len(config.ListenAddress) > 256 || len(config.GatewayListenAddress) > 256 || len(config.DatabaseURL) > 4096 || len(config.Environment) > 32 || len(config.GatewayCertificateFile) > 4096 || len(config.GatewayPrivateKeyFile) > 4096 || len(config.GatewayCAFile) > 4096 {
		return Config{}, errors.New("runtime configuration value is too long")
	}
	return config, nil
}
