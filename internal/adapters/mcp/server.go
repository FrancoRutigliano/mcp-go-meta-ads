// Package mcp es el adaptador de entrada: expone los casos de uso como tools del
// protocolo MCP y traduce sus resultados/errores a mensajes en español.
package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/mashats/meta-ads-manager/internal/app"
)

// NewServer construye el servidor MCP con las dos tools de lectura registradas.
func NewServer(name, version string, lc *app.ListCampaigns, gi *app.GetInsights) *server.MCPServer {
	s := server.NewMCPServer(
		name,
		version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	s.AddTool(campaignsTool(), campaignsHandler(lc))
	s.AddTool(insightsTool(), insightsHandler(gi))

	return s
}
