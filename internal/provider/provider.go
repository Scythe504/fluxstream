package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const ProviderUrl = "http://localhost:8081"

type Provider struct {
	Name         string
	BaseUrl      *url.URL
	MediaSupport MediaType
	Proxy        *httputil.ReverseProxy
}

type Manager struct {
	providers sync.Map
}

func NewManager() *Manager {
	return &Manager{}
}

type RegistryProvider struct {
	ID                  string `json:"id"`
	ProviderName        string `json:"provider_name"`
	ProviderURL         string `json:"provider_url"`
	VerificationPending bool   `json:"verification_pending"`
	Version             string `json:"version"`
	VerifiedAt          *int64 `json:"verified_at,omitempty"`
	ProviderType        string `json:"provider_type"`
	CreatedAt           int64  `json:"created_at"`
}

// Get retrieves a provider by name, querying the registry if it is not cached.
func (m *Manager) Get(ctx context.Context, name string) (*Provider, error) {
	if p, ok := m.providers.Load(name); ok {
		return p.(*Provider), nil
	}

	targetURL, providerType, err := m.fetchProviderURL(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("failed to locate provider URL: %w", err)
	}

	pr := InitProvider(name, targetURL, MediaType(providerType))
	if pr == nil {
		return nil, fmt.Errorf("failed to initialize provider %s at %s", name, targetURL)
	}

	actual, loaded := m.providers.LoadOrStore(name, pr)
	if loaded {
		return actual.(*Provider), nil
	}
	return pr, nil
}

func (m *Manager) fetchProviderURL(ctx context.Context, name string) (string, string, error) {
	providersURL := os.Getenv("PROVIDERS_SERVICE_URL")
	if providersURL == "" {
		providersURL = "http://localhost:8082"
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	reqURL := fmt.Sprintf("%s/api/providers", providersURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("registry returned status: %d", resp.StatusCode)
	}

	var allProviders []RegistryProvider
	if err := json.NewDecoder(resp.Body).Decode(&allProviders); err != nil {
		return "", "", err
	}

	for _, p := range allProviders {
		if strings.EqualFold(p.ProviderName, name) {
			return p.ProviderURL, p.ProviderType, nil
		}
	}

	return "", "", fmt.Errorf("provider %s not found in registry", name)
}

func InitProvider(name string, baseURL string, providerMediaType MediaType) *Provider {
	targetURL, _ := url.Parse(baseURL)

	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 20 * time.Second,
		},
	}

	var mediaType MediaType
	switch providerMediaType {
	case MediaTypeAnime:
		mediaType = MediaTypeAnime
	case MediaTypeMovie:
		mediaType = MediaTypeMovie
	case MediaTypeSeries:
		mediaType = MediaTypeSeries
	default:
		mediaType = MediaTypeSeries
	}

	p := &Provider{
		Name:         name,
		BaseUrl:      targetURL,
		MediaSupport: mediaType,
		Proxy:        proxy,
	}

	proxy.Rewrite = func(req *httputil.ProxyRequest) {
		req.SetURL(p.BaseUrl)
		remainingPath := strings.TrimPrefix(req.In.URL.Path, "/api/providers/"+p.Name)
		req.Out.URL.Path = "/api" + remainingPath
		req.Out.URL.RawQuery = req.In.URL.RawQuery
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Del("Access-Control-Allow-Origin")
		resp.Header.Del("Access-Control-Allow-Methods")
		resp.Header.Del("Access-Control-Allow-Headers")
		resp.Header.Del("Access-Control-Allow-Credentials")
		return nil
	}

	return p
}
