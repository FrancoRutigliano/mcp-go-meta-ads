// Package meta es el adaptador de salida hacia la Meta Graph API. Es la ÚNICA
// pieza del sistema que conoce el token y habla con Meta (Constitución III).
// Traduce respuestas a entidades del dominio y errores a errores semánticos.
package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mashats/meta-ads-manager/internal/domain"
	"github.com/mashats/meta-ads-manager/internal/ports"
)

const (
	defaultBaseURL    = "https://graph.facebook.com"
	defaultInterval   = 200 * time.Millisecond
	defaultMaxRetries = 2
	defaultBackoff    = 500 * time.Millisecond

	campaignFields = "id,name,status,objective"
	insightFields  = "campaign_id,campaign_name,spend,impressions,clicks,reach,ctr,cpc,date_start,date_stop," +
		"frequency,inline_link_clicks,inline_link_click_ctr,purchase_roas,actions,action_values,cost_per_action_type"
)

// Client implementa ports.MetaReader contra la Graph API.
type Client struct {
	http       *http.Client
	baseURL    string
	apiVersion string
	accountID  string
	token      string
	limiter    *Limiter
	maxRetries int
	backoff    time.Duration
}

// Compile-time: el cliente satisface el puerto del dominio.
var _ ports.MetaReader = (*Client)(nil)

// Option configura el cliente.
type Option func(*Client)

// WithBaseURL sobreescribe el host de la Graph API (tests con httptest).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithHTTPClient inyecta un *http.Client propio.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

// WithLimiter inyecta un limiter (tests usan intervalo 0).
func WithLimiter(l *Limiter) Option { return func(c *Client) { c.limiter = l } }

// WithRetry configura los reintentos ante rate limiting y el backoff base.
func WithRetry(maxRetries int, backoff time.Duration) Option {
	return func(c *Client) {
		c.maxRetries = maxRetries
		c.backoff = backoff
	}
}

// New construye el cliente con la credencial y la cuenta objetivo. El token se
// guarda en memoria y nunca se expone en logs ni en errores.
func New(token, accountID, apiVersion string, opts ...Option) *Client {
	c := &Client{
		http:       &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
		apiVersion: apiVersion,
		accountID:  accountID,
		token:      token,
		limiter:    NewLimiter(defaultInterval),
		maxRetries: defaultMaxRetries,
		backoff:    defaultBackoff,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ListCampaigns implementa ports.MetaReader.
func (c *Client) ListCampaigns(ctx context.Context, q domain.CampaignQuery) ([]domain.Campaign, error) {
	const op = "meta.ListCampaigns"

	params := url.Values{}
	params.Set("fields", campaignFields)
	if q.Limit > 0 {
		params.Set("limit", strconv.Itoa(q.Limit))
	}
	if q.OnlyActive {
		params.Set("effective_status", `["ACTIVE"]`)
	}

	body, err := c.get(ctx, c.accountID+"/campaigns", params, op)
	if err != nil {
		return nil, err
	}
	campaigns, err := parseCampaigns(body)
	if err != nil {
		return nil, domain.NewError(domain.KindUpstream, op, fmt.Errorf("parse: %w", err))
	}
	return campaigns, nil
}

// GetInsights implementa ports.MetaReader. CampaignID vacío consulta a nivel de
// cuenta con desglose por campaña.
func (c *Client) GetInsights(ctx context.Context, q domain.InsightQuery) ([]domain.Insight, error) {
	const op = "meta.GetInsights"

	if err := q.Range.Valid(); err != nil {
		return nil, domain.NewError(domain.KindInvalidInput, op, err)
	}

	node := c.accountID
	if q.CampaignID != "" {
		node = q.CampaignID
	}

	timeRange, err := json.Marshal(map[string]string{
		"since": q.Range.Since.Format(graphDateLayout),
		"until": q.Range.Until.Format(graphDateLayout),
	})
	if err != nil {
		return nil, domain.NewError(domain.KindUpstream, op, err)
	}

	params := url.Values{}
	params.Set("level", "campaign")
	params.Set("fields", insightFields)
	params.Set("time_range", string(timeRange))

	body, err := c.get(ctx, node+"/insights", params, op)
	if err != nil {
		return nil, err
	}
	insights, err := parseInsights(body)
	if err != nil {
		return nil, domain.NewError(domain.KindUpstream, op, fmt.Errorf("parse: %w", err))
	}
	return insights, nil
}

// GetAudienceBreakdown implementa ports.MetaReader. Segmenta el rendimiento por
// la dimensión pedida usando el parámetro breakdowns de la Graph API.
func (c *Client) GetAudienceBreakdown(ctx context.Context, q domain.AudienceQuery) (domain.AudienceBreakdown, error) {
	const op = "meta.GetAudienceBreakdown"

	if !q.Dimension.Valid() {
		return domain.AudienceBreakdown{}, domain.NewError(domain.KindInvalidInput, op,
			fmt.Errorf("dimensión no soportada: %q", q.Dimension))
	}
	if err := q.Range.Valid(); err != nil {
		return domain.AudienceBreakdown{}, domain.NewError(domain.KindInvalidInput, op, err)
	}

	node := c.accountID
	if q.CampaignID != "" {
		node = q.CampaignID
	}

	timeRange, err := json.Marshal(map[string]string{
		"since": q.Range.Since.Format(graphDateLayout),
		"until": q.Range.Until.Format(graphDateLayout),
	})
	if err != nil {
		return domain.AudienceBreakdown{}, domain.NewError(domain.KindUpstream, op, err)
	}

	params := url.Values{}
	params.Set("fields", insightFields)
	params.Set("breakdowns", string(q.Dimension))
	params.Set("time_range", string(timeRange))

	body, err := c.get(ctx, node+"/insights", params, op)
	if err != nil {
		return domain.AudienceBreakdown{}, err
	}
	segments, err := parseBreakdown(body, q.Dimension)
	if err != nil {
		return domain.AudienceBreakdown{}, domain.NewError(domain.KindUpstream, op, fmt.Errorf("parse: %w", err))
	}
	return domain.AudienceBreakdown{
		Dimension: q.Dimension,
		Range:     q.Range,
		Segments:  segments,
	}, nil
}

// get ejecuta una solicitud GET con rate limiting y reintentos con backoff ante
// respuestas de rate limiting (Constitución, Principio VI).
func (c *Client) get(ctx context.Context, path string, params url.Values, op string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, domain.NewError(domain.KindUpstream, op, err)
		}

		body, err := c.do(ctx, path, params, op)
		if err == nil {
			return body, nil
		}
		lastErr = err

		if domain.KindOf(err) == domain.KindRateLimited && attempt < c.maxRetries {
			select {
			case <-time.After(c.backoffFor(attempt)):
				continue
			case <-ctx.Done():
				return nil, domain.NewError(domain.KindUpstream, op, ctx.Err())
			}
		}
		return nil, err
	}
	return nil, lastErr
}

func (c *Client) do(ctx context.Context, path string, params url.Values, op string) ([]byte, error) {
	// El token va en la query pero nunca se loguea: no registramos URLs.
	full := c.baseURL + "/" + c.apiVersion + "/" + path
	q := url.Values{}
	maps.Copy(q, params)
	q.Set("access_token", c.token)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, full+"?"+q.Encode(), nil)
	if err != nil {
		return nil, domain.NewError(domain.KindUpstream, op, err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, domain.NewError(domain.KindUpstream, op, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, domain.NewError(domain.KindUpstream, op, err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var env errorEnvelope
		_ = json.Unmarshal(body, &env) // si no parsea, env.Error queda nil
		return nil, translateError(resp.StatusCode, env.Error, op)
	}
	return body, nil
}

func (c *Client) backoffFor(attempt int) time.Duration {
	// Backoff exponencial: base * 2^attempt.
	return c.backoff * (1 << attempt)
}
