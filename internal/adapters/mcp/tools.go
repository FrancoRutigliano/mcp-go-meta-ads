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

// proposeCampaignStatusHandler construye el handler de propose_campaign_status.
// Es el paso SIN efecto (Principio II): calcula el cambio y devuelve un id.
func proposeCampaignStatusHandler(uc *app.ProposeCampaignStatus) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		campaignID := req.GetString("campaign_id", "")
		action := domain.CampaignAction(req.GetString("action", ""))

		p, err := uc.Execute(ctx, campaignID, action)
		if err != nil {
			slog.Error("propose_campaign_status falló", "tool", "propose_campaign_status",
				"kind", domain.KindOf(err), "error", err.Error())
			return mcp.NewToolResultError(messageForError(err)), nil
		}
		return mcp.NewToolResultText(formatProposal(p)), nil
	}
}

// confirmActionHandler construye el handler de confirm_action. Es el ÚNICO paso
// de escritura, y sólo aplica propuestas existentes (Principio II).
func confirmActionHandler(uc *app.ConfirmProposal) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		proposalID := req.GetString("proposal_id", "")
		confirmedBy := req.GetString("confirmed_by", "")

		p, err := uc.Execute(ctx, proposalID, confirmedBy)
		if err != nil {
			slog.Error("confirm_action falló", "tool", "confirm_action",
				"kind", domain.KindOf(err), "error", err.Error())
			return mcp.NewToolResultError(messageForError(err)), nil
		}
		return mcp.NewToolResultText(formatConfirmation(p)), nil
	}
}

// adPerformanceHandler construye el handler de la tool get_ad_performance.
func adPerformanceHandler(uc *app.GetAdPerformance) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		campaignID := req.GetString("campaign_id", "")
		limit := req.GetInt("limit", 0)
		since := req.GetString("since", "")
		until := req.GetString("until", "")

		rng, ok := parseOptionalRange(since, until)
		if !ok {
			return mcp.NewToolResultError(
				"Indicá ambas fechas (desde y hasta) en formato AAAA-MM-DD, o ninguna para usar los últimos 30 días.",
			), nil
		}

		reports, applied, err := uc.Execute(ctx, campaignID, rng, limit)
		if err != nil {
			slog.Error("get_ad_performance falló", "tool", "get_ad_performance",
				"kind", domain.KindOf(err), "error", err.Error())
			return mcp.NewToolResultError(messageForError(err)), nil
		}
		return mcp.NewToolResultText(formatAdPerformance(reports, applied)), nil
	}
}

// funnelHandler construye el handler de la tool get_conversion_funnel.
func funnelHandler(uc *app.GetFunnel) server.ToolHandlerFunc {
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

		funnels, applied, err := uc.Execute(ctx, campaignID, rng)
		if err != nil {
			slog.Error("get_conversion_funnel falló", "tool", "get_conversion_funnel",
				"kind", domain.KindOf(err), "error", err.Error())
			return mcp.NewToolResultError(messageForError(err)), nil
		}
		return mcp.NewToolResultText(formatFunnel(funnels, applied)), nil
	}
}

// audienceHandler construye el handler de la tool get_audience_breakdown.
func audienceHandler(uc *app.GetAudienceBreakdown) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		campaignID := req.GetString("campaign_id", "")
		dim := domain.BreakdownDimension(req.GetString("dimension", ""))
		since := req.GetString("since", "")
		until := req.GetString("until", "")

		rng, ok := parseOptionalRange(since, until)
		if !ok {
			return mcp.NewToolResultError(
				"Indicá ambas fechas (desde y hasta) en formato AAAA-MM-DD, o ninguna para usar los últimos 30 días.",
			), nil
		}

		br, applied, err := uc.Execute(ctx, campaignID, dim, rng)
		if err != nil {
			slog.Error("get_audience_breakdown falló", "tool", "get_audience_breakdown",
				"kind", domain.KindOf(err), "error", err.Error())
			return mcp.NewToolResultError(messageForError(err)), nil
		}
		return mcp.NewToolResultText(formatBreakdown(br, applied)), nil
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
		// Es de solo lectura: no modifica nada (Constitución, solo lectura).
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
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
		// Es de solo lectura: no modifica nada (Constitución, solo lectura).
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
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

// proposeCampaignStatusTool define el esquema de propose_campaign_status.
func proposeCampaignStatusTool() mcp.Tool {
	return mcp.NewTool("propose_campaign_status",
		mcp.WithDescription("Paso 1 de 2 (SIN efecto): propone pausar o activar una campaña y devuelve un proposal_id. NO cambia nada en Meta; para aplicar el cambio hay que confirmar con la tool confirm_action."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithString("campaign_id",
			mcp.Required(),
			mcp.Description("ID de la campaña a pausar o activar."),
		),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description("Acción a proponer: 'pause' (pausar) o 'activate' (activar)."),
			mcp.Enum("pause", "activate"),
		),
	)
}

// confirmActionTool define el esquema de confirm_action. Es DESTRUCTIVA: aplica
// un cambio real e irreversible sobre la cuenta.
func confirmActionTool() mcp.Tool {
	return mcp.NewTool("confirm_action",
		mcp.WithDescription("Paso 2 de 2 (APLICA el cambio): ejecuta una propuesta creada antes con una tool propose_*. Requiere el proposal_id devuelto por el paso propose. Afecta la cuenta de forma real e irreversible."),
		mcp.WithReadOnlyHintAnnotation(false),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithIdempotentHintAnnotation(false),
		mcp.WithOpenWorldHintAnnotation(true),
		mcp.WithString("proposal_id",
			mcp.Required(),
			mcp.Description("El proposal_id devuelto por el paso propose."),
		),
		mcp.WithString("confirmed_by",
			mcp.Description("Quién confirma el cambio (queda registrado en la auditoría). Opcional."),
		),
	)
}

// adPerformanceTool define el esquema de la tool get_ad_performance.
func adPerformanceTool() mcp.Tool {
	return mcp.NewTool("get_ad_performance",
		mcp.WithDescription("Lista el rendimiento por anuncio (creativo), ordenado de mejor a peor por ROAS, con ROAS, CPA, compras, facturación, CTR de enlace y frecuencia. Sirve para ver qué anuncio funciona y cuál no. Si no se indica período, usa los últimos 30 días."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("campaign_id",
			mcp.Description("ID de una campaña específica. Si se omite, lista anuncios de toda la cuenta."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Cantidad máxima de anuncios a devolver. Por defecto 25."),
		),
		mcp.WithString("since",
			mcp.Description("Fecha de inicio del período en formato AAAA-MM-DD. Opcional (junto con 'until')."),
		),
		mcp.WithString("until",
			mcp.Description("Fecha de fin del período en formato AAAA-MM-DD. Opcional (junto con 'since')."),
		),
	)
}

// funnelTool define el esquema de la tool get_conversion_funnel.
func funnelTool() mcp.Tool {
	return mcp.NewTool("get_conversion_funnel",
		mcp.WithDescription("Muestra el embudo de conversión (impresiones → clics → vistas de página → carrito → inicio de pago → compras) y resalta en qué paso se cae la gente. Sirve para responder \"¿por qué no vende?\". Si no se indica período, usa los últimos 30 días."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("campaign_id",
			mcp.Description("ID de una campaña específica. Si se omite, arma el embudo a nivel de toda la cuenta."),
		),
		mcp.WithString("since",
			mcp.Description("Fecha de inicio del período en formato AAAA-MM-DD. Opcional (junto con 'until')."),
		),
		mcp.WithString("until",
			mcp.Description("Fecha de fin del período en formato AAAA-MM-DD. Opcional (junto con 'since')."),
		),
	)
}

// audienceTool define el esquema de la tool get_audience_breakdown.
func audienceTool() mcp.Tool {
	return mcp.NewTool("get_audience_breakdown",
		mcp.WithDescription("Desglosa el rendimiento (ROAS, CPA, compras, facturación, CTR de enlace, frecuencia) por una dimensión de audiencia: edad, género, región, plataforma o posición del anuncio. Si no se indica período, usa los últimos 30 días."),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithDestructiveHintAnnotation(false),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithOpenWorldHintAnnotation(false),
		mcp.WithString("dimension",
			mcp.Required(),
			mcp.Description("Dimensión por la que segmentar."),
			mcp.Enum("age", "gender", "region", "publisher_platform", "platform_position"),
		),
		mcp.WithString("campaign_id",
			mcp.Description("ID de una campaña específica. Si se omite, desglosa a nivel de toda la cuenta."),
		),
		mcp.WithString("since",
			mcp.Description("Fecha de inicio del período en formato AAAA-MM-DD. Opcional (junto con 'until')."),
		),
		mcp.WithString("until",
			mcp.Description("Fecha de fin del período en formato AAAA-MM-DD. Opcional (junto con 'since')."),
		),
	)
}
