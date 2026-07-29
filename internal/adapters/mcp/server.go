// Package mcp es el adaptador de entrada: expone los casos de uso como tools del
// protocolo MCP y traduce sus resultados/errores a mensajes en español.
package mcp

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/mashats/meta-ads-manager/internal/app"
)

// Deps reúne los casos de uso que el servidor MCP expone como tools.
type Deps struct {
	ListCampaigns *app.ListCampaigns
	Insights      *app.GetInsights
	Audience      *app.GetAudienceBreakdown
	Funnel        *app.GetFunnel
	AdPerformance *app.GetAdPerformance
	Budgets       *app.GetBudgets
	ProposeStatus *app.ProposeCampaignStatus
	ProposeBudget *app.ProposeBudget
	Confirm       *app.ConfirmProposal
}

// NewServer construye el servidor MCP con las tools registradas: lectura y el
// par propose/confirm para escritura (Constitución, Principio II).
func NewServer(name, version string, d Deps) *server.MCPServer {
	s := server.NewMCPServer(
		name,
		version,
		server.WithToolCapabilities(true),
		server.WithRecovery(),
	)

	// Lectura.
	s.AddTool(campaignsTool(), campaignsHandler(d.ListCampaigns))
	s.AddTool(insightsTool(), insightsHandler(d.Insights))
	s.AddTool(audienceTool(), audienceHandler(d.Audience))
	s.AddTool(funnelTool(), funnelHandler(d.Funnel))
	s.AddTool(adPerformanceTool(), adPerformanceHandler(d.AdPerformance))
	s.AddTool(budgetsTool(), budgetsHandler(d.Budgets))

	// Escritura: dos pasos (propose sin efecto → confirm aplica).
	s.AddTool(proposeCampaignStatusTool(), proposeCampaignStatusHandler(d.ProposeStatus))
	s.AddTool(proposeBudgetTool(), proposeBudgetHandler(d.ProposeBudget))
	s.AddTool(confirmActionTool(), confirmActionHandler(d.Confirm))

	return s
}
