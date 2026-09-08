package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"personalvpn/services/api/internal/agent"
	"personalvpn/services/api/internal/auth"
	"personalvpn/services/api/internal/config"
	"personalvpn/services/api/internal/httpapi"
	"personalvpn/services/api/internal/postgres"
)

func main() {
	runtimeConfig, err := config.Load(os.Getenv)
	if err != nil {
		slog.Error("invalid runtime configuration", "error", err)
		os.Exit(1)
	}
	var database *postgres.Store
	if passwordFile := os.Getenv("DATABASE_PASSWORD_FILE"); passwordFile != "" {
		database, err = postgres.OpenWithPasswordFile(context.Background(), runtimeConfig.DatabaseURL, passwordFile)
	} else {
		database, err = postgres.Open(context.Background(), runtimeConfig.DatabaseURL)
	}
	if err != nil {
		slog.Error("open database failed")
		os.Exit(1)
	}
	defer database.Close()

	authService := auth.NewService(database, nil)
	server := httpapi.NewServer(runtimeConfig.ListenAddress, httpapi.NewMux(authService, database, database, database))
	var gatewayServer *http.Server
	if runtimeConfig.GatewayListenAddress != "" {
		tlsConfig, tlsErr := agent.ServerTLSConfig(agent.TLSFiles{CertificateFile: runtimeConfig.GatewayCertificateFile, PrivateKeyFile: runtimeConfig.GatewayPrivateKeyFile, CAFile: runtimeConfig.GatewayCAFile})
		if tlsErr != nil {
			slog.Error("invalid gateway TLS configuration", "error", tlsErr)
			os.Exit(1)
		}
		gatewayServer = httpapi.NewMTLSServer(runtimeConfig.GatewayListenAddress, httpapi.NewMux(nil, nil, database, database), tlsConfig)
		go func() {
			slog.Info("gateway mTLS listener", "address", gatewayServer.Addr)
			if err := gatewayServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				slog.Error("gateway listener stopped", "error", err)
				os.Exit(1)
			}
		}()
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-shutdownContext.Done()
		closeContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(closeContext); err != nil {
			slog.Error("shutdown control API", "error", err)
		}
		if gatewayServer != nil {
			_ = gatewayServer.Shutdown(closeContext)
		}
	}()

	slog.Info("control API listening", "address", server.Addr, "environment", runtimeConfig.Environment)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("control API stopped", "error", err)
		os.Exit(1)
	}
}
