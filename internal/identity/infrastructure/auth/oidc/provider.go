package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type Provider struct {
	issuer      string
	clientID    string
	httpClient  *http.Client
	mu          sync.RWMutex
	wellKnown   *wellKnown
	wellKnownAt time.Time
}

type wellKnown struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	Issuer                string `json:"issuer"`
}

func New(issuer, clientID string) *Provider {
	return &Provider{
		issuer:     strings.TrimRight(issuer, "/"),
		clientID:   clientID,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (p *Provider) ExchangeCode(ctx context.Context, code string) (string, string, error) {
	wk, err := p.getWellKnown(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get OIDC config: %w", err)
	}

	resp, err := p.httpClient.PostForm(wk.TokenEndpoint, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"client_id":    {p.clientID},
		"redirect_uri": {p.redirectURI()},
	})
	if err != nil {
		return "", "", fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("oidc token exchange failed: %s", string(body))
	}

	var result struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return result.AccessToken, result.IDToken, nil
}

func (p *Provider) getWellKnown(ctx context.Context) (*wellKnown, error) {
	p.mu.RLock()
	if p.wellKnown != nil && time.Since(p.wellKnownAt) < 5*time.Minute {
		wk := p.wellKnown
		p.mu.RUnlock()
		return wk, nil
	}
	p.mu.RUnlock()

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.wellKnown != nil && time.Since(p.wellKnownAt) < 5*time.Minute {
		return p.wellKnown, nil
	}

	wellKnownURL := p.issuer + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnownURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OIDC discovery: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery returned status %d", resp.StatusCode)
	}

	var wk wellKnown
	if err := json.NewDecoder(resp.Body).Decode(&wk); err != nil {
		return nil, fmt.Errorf("failed to decode OIDC discovery: %w", err)
	}

	p.wellKnown = &wk
	p.wellKnownAt = time.Now()
	return &wk, nil
}

func (p *Provider) redirectURI() string {
	return ""
}

func (p *Provider) AuthorizationURL(state string) string {
	return fmt.Sprintf("%s?response_type=code&client_id=%s&state=%s&scope=openid+email+profile&redirect_uri=%s",
		p.issuer,
		p.clientID,
		state,
		url.QueryEscape(p.redirectURI()),
	)
}
