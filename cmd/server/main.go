// Command server arranca el servidor MCP de Meta Ads.
//
// Composición de la arquitectura hexagonal:
//
//	config → adaptador meta (salida) → casos de uso → adaptador mcp (entrada).
//
// Arranque fail-fast: si falta el token o la cuenta, aborta con código 1
// (Constitución, Principio I).
package main

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/server"

	mcpadapter "github.com/mashats/meta-ads-manager/internal/adapters/mcp"
	"github.com/mashats/meta-ads-manager/internal/adapters/meta"
	"github.com/mashats/meta-ads-manager/internal/app"
	"github.com/mashats/meta-ads-manager/internal/config"
)

const (
	serverName    = "meta-ads-manager"
	serverVersion = "0.1.0"
)

func main() {
	// Log estructurado en JSON (Constitución, Principio IV/V: trazabilidad).
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// En local cargamos .env si existe; en contenedor/PaaS las variables ya
	// vienen inyectadas y godotenv.Load simplemente no encuentra archivo.
	_ = godotenv.Load()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		// Mensaje claro y salida inmediata: nunca arrancar degradado.
		slog.Error("configuración inválida; no se puede arrancar", "error", err)
		os.Exit(1)
	}
	// cfg.String() redacta el token: nunca se loguea el secreto.
	slog.Info("configuración cargada", "config", cfg.String())

	// Adaptador de salida: única pieza que habla con Meta (Principio III).
	reader := meta.New(cfg.AccessToken, cfg.AccountID, cfg.APIVersion)

	// Casos de uso (umbrales y suficiencia inyectados desde config).
	listCampaigns := app.NewListCampaigns(reader)
	getInsights := app.NewGetInsights(reader, cfg.Thresholds, cfg.Sufficiency)
	getAudienceBreakdown := app.NewGetAudienceBreakdown(reader, cfg.Thresholds, cfg.Sufficiency)

	// Adaptador de entrada: tools MCP.
	mcpServer := mcpadapter.NewServer(serverName, serverVersion, listCampaigns, getInsights, getAudienceBreakdown)

	// Transporte Streamable HTTP sobre $PORT (apto para contenedor/Railway).
	httpServer := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath(cfg.Endpoint),
	)

	addr := ":" + cfg.Port
	slog.Info("servidor MCP escuchando", "addr", addr, "endpoint", cfg.Endpoint,
		"account", cfg.AccountID, "api_version", cfg.APIVersion)

	if err := httpServer.Start(addr); err != nil {
		slog.Error("el servidor terminó con error", "error", err)
		os.Exit(1)
	}
}
