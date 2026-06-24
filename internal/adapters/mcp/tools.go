package mcp

import (
	"context"
	"log/slog"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/mashats/meta-ads-manager/internal/app"
	"github.com/mashats/meta-ads-manager/internal/domain"
)

const dateLayout = "2006-01-02"

// campaignsHandler construye el handler de la tool get_campaigns.
func campaignsHandler(lc *app.ListCampaigns) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status := req.GetString("status", "active")
		limit := req.GetInt("limit", 0)
		onlyActive := status != "all"

		campaigns, err := lc.Execute(ctx, onlyActive, limit)
		if err != nil {
			// Error semántico → mensaje en español. El detalle técnico se loguea
			// del lado del servidor, separado del mensaje (Constitución V y VII).
			slog.Error("get_campaigns falló", "tool", "get_campaigns",
				"kind", domain.KindOf(err), "error", err.Error())
			return mcp.NewToolResultError(messageForError(err)), nil
		}

		effectiveLimit := limit
		if effectiveLimit <= 0 {
			effectiveLimit = app.DefaultCampaignLimit
		}
		truncated := len(campaigns) >= effectiveLimit

		return mcp.NewToolResultText(formatCampaigns(campaigns, truncated)), nil
	}
}

// insightsHandler construye el handler de la tool get_campaigns_insights.
func insightsHandler(gi *app.GetInsights) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		campaignID := req.GetString("campaign_id", "")
		since := req.GetString("since", "")
		until := req.GetString("until", "")

		rng, ok := parseOptionalRange(since, until)
		if !ok {
			return mcp.NewToolResultError(
				"Indicá ambas fechas (desde y hasta) en formato AAAA-MM-DD, o ninguna para usar los últimos 30 días.",
			), nil
		}

		insights, applied, err := gi.Execute(ctx, campaignID, rng)
		if err != nil {
			slog.Error("get_campaigns_insights falló", "tool", "get_campaigns_insights",
				"kind", domain.KindOf(err), "error", err.Error())
			return mcp.NewToolResultError(messageForError(err)), nil
		}
		return mcp.NewToolResultText(formatInsights(insights, applied)), nil
	}
}

// parseOptionalRange interpreta las fechas opcionales. Devuelve (nil, true)
// cuando no se pasó ninguna (se usará el default). Devuelve (nil, false) si el
// par está incompleto o mal formado.
func parseOptionalRange(since, until string) (*domain.DateRange, bool) {
	if since == "" && until == "" {
		return nil, true
	}
	if since == "" || until == "" {
		return nil, false
	}
	s, err1 := time.Parse(dateLayout, since)
	u, err2 := time.Parse(dateLayout, until)
	if err1 != nil || err2 != nil {
		return nil, false
	}
	return &domain.DateRange{Since: s, Until: u}, true
}

// campaignsTool define el esquema de la tool get_campaigns.
func campaignsTool() mcp.Tool {
	return mcp.NewTool("get_campaigns",
		mcp.WithDescription("Lista las campañas de la cuenta publicitaria de Meta. Por defecto sólo las activas."),
		mcp.WithString("status",
			mcp.Description("Filtro de estado: 'active' (sólo activas, por defecto) o 'all' (todas)."),
			mcp.Enum("active", "all"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Cantidad máxima de campañas a devolver. Por defecto 50."),
		),
	)
}

// insightsTool define el esquema de la tool get_campaigns_insights.
func insightsTool() mcp.Tool {
	return mcp.NewTool("get_campaigns_insights",
		mcp.WithDescription("Devuelve el rendimiento (gasto, impresiones, clics, alcance, CTR, CPC) de las campañas para un período. Si no se indica período, usa los últimos 30 días."),
		mcp.WithString("campaign_id",
			mcp.Description("ID de una campaña específica. Si se omite, devuelve todas las campañas de la cuenta."),
		),
		mcp.WithString("since",
			mcp.Description("Fecha de inicio del período en formato AAAA-MM-DD. Opcional (junto con 'until')."),
		),
		mcp.WithString("until",
			mcp.Description("Fecha de fin del período en formato AAAA-MM-DD. Opcional (junto con 'since')."),
		),
	)
}
