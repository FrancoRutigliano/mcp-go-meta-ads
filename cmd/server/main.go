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
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/server"

	mcpadapter "github.com/mashats/meta-ads-manager/internal/adapters/mcp"
	"github.com/mashats/meta-ads-manager/internal/adapters/memstore"
	"github.com/mashats/meta-ads-manager/internal/adapters/meta"
	"github.com/mashats/meta-ads-manager/internal/adapters/oauth"
	"github.com/mashats/meta-ads-manager/internal/app"
	"github.com/mashats/meta-ads-manager/internal/config"
)

const (
	serverName    = "meta-ads-manager"
	serverVersion = "5.0.0"
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
	getFunnel := app.NewGetFunnel(reader)
	getAdPerformance := app.NewGetAdPerformance(reader, cfg.Thresholds, cfg.Sufficiency)
	getBudgets := app.NewGetBudgets(reader)

	// Escritura: store de propuestas en memoria + par propose/confirm. El mismo
	// cliente Meta implementa lectura y escritura; el confirm audita vía slog.
	proposals := memstore.New()
	proposeStatus := app.NewProposeCampaignStatus(reader, proposals)
	proposeBudget := app.NewProposeBudget(reader, proposals, cfg.Guardrails, cfg.Thresholds, cfg.Sufficiency)
	confirm := app.NewConfirmProposal(proposals, reader, reader, slog.Default())

	// Adaptador de entrada: tools MCP.
	mcpServer := mcpadapter.NewServer(serverName, serverVersion, mcpadapter.Deps{
		ListCampaigns: listCampaigns,
		Insights:      getInsights,
		Audience:      getAudienceBreakdown,
		Funnel:        getFunnel,
		AdPerformance: getAdPerformance,
		Budgets:       getBudgets,
		ProposeStatus: proposeStatus,
		ProposeBudget: proposeBudget,
		Confirm:       confirm,
	})

	// Transporte Streamable HTTP sobre $PORT (apto para contenedor/Railway).
	streamable := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath(cfg.Endpoint),
	)

	// Auth del endpoint. OAuth es el camino principal; el bearer estático
	// sobrevive como puente durante la migración. Cada modo se anuncia de forma
	// explícita: un endpoint abierto nunca debe quedar abierto en silencio.
	ctx, cancelKeys := context.WithCancel(context.Background())
	defer cancelKeys()

	verifier, err := buildVerifier(ctx, cfg.OAuth)
	if err != nil {
		slog.Error("no se pudo inicializar OAuth; no se puede arrancar", "error", err)
		os.Exit(1)
	}
	logAuthMode(cfg, verifier != nil)

	authCfg := mcpadapter.AuthConfig{
		ResourceMetadataURL: cfg.OAuth.MetadataURL(),
		RequiredScopes:      cfg.OAuth.RequiredScopes,
		StaticToken:         cfg.AuthToken,
	}
	// Una interfaz nil-typed no es una interfaz nil: sólo la asignamos cuando
	// hay verificador de verdad, o el middleware creería que OAuth está activo.
	if verifier != nil {
		authCfg.Verifier = verifier
	}

	handler := mcpadapter.NewRouter(mcpadapter.RouterConfig{
		Endpoint:   cfg.Endpoint,
		MCPHandler: streamable,
		Auth:       authCfg,
		Metadata:   metadataConfig(cfg.OAuth),
	})

	addr := ":" + cfg.Port
	slog.Info("servidor MCP escuchando", "addr", addr, "endpoint", cfg.Endpoint,
		"account", cfg.AccountID, "api_version", cfg.APIVersion)

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	if err := httpServer.ListenAndServe(); err != nil {
		slog.Error("el servidor terminó con error", "error", err)
		os.Exit(1)
	}
}

// buildVerifier arma el verificador de access tokens, o devuelve nil si OAuth
// no está configurado.
//
// El descubrimiento y la primera carga del JWKS ocurren acá, en el arranque:
// si el IdP no responde, preferimos no levantar antes que levantar un servidor
// que va a rechazar todo sin poder explicar por qué.
func buildVerifier(ctx context.Context, cfg config.OAuthConfig) (*oauth.Verifier, error) {
	if !cfg.Enabled() {
		return nil, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}

	jwksURI, err := oauth.DiscoverJWKSURI(ctx, client, cfg.Issuer)
	if err != nil {
		return nil, err
	}
	slog.Info("authorization server descubierto", "issuer", cfg.Issuer, "jwks_uri", jwksURI)

	keys, err := oauth.NewCachedKeySet(ctx, jwksURI, client)
	if err != nil {
		return nil, err
	}

	return oauth.NewVerifier(oauth.Config{
		Issuer:         cfg.Issuer,
		Resource:       cfg.Resource,
		RequiredScopes: cfg.RequiredScopes,
	}, keys)
}

// logAuthMode deja asentado en el arranque con qué credenciales se protege el
// endpoint. Es la única forma de notar desde afuera que quedó abierto.
func logAuthMode(cfg *config.Config, oauthOn bool) {
	switch {
	case oauthOn && cfg.AuthToken != "":
		slog.Warn("OAuth habilitado, pero MCP_AUTH_TOKEN sigue activo como credencial estática; "+
			"quitá la variable cuando termine la migración",
			"resource", cfg.OAuth.Resource, "issuer", cfg.OAuth.Issuer)
	case oauthOn:
		slog.Info("autenticación OAuth habilitada",
			"resource", cfg.OAuth.Resource,
			"issuer", cfg.OAuth.Issuer,
			"metadata", cfg.OAuth.MetadataURL())
	case cfg.AuthToken != "":
		slog.Warn("sólo bearer estático (MCP_AUTH_TOKEN): el conector de Claude no puede autenticarse así; " +
			"configurá OAUTH_ISSUER y PUBLIC_URL")
	default:
		slog.Warn("sin OAUTH_ISSUER ni MCP_AUTH_TOKEN: el endpoint queda ABIERTO (sin autenticación)")
	}
}

// metadataConfig arma el documento de descubrimiento, o nil si OAuth no está
// configurado.
func metadataConfig(cfg config.OAuthConfig) *mcpadapter.MetadataConfig {
	if !cfg.Enabled() {
		return nil
	}
	return &mcpadapter.MetadataConfig{
		Path:                 cfg.MetadataPath(),
		Resource:             cfg.Resource,
		AuthorizationServers: []string{cfg.Issuer},
		ScopesSupported:      cfg.RequiredScopes,
		ResourceName:         serverName,
	}
}
