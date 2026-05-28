package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/scythe504/fluxstream/internal/utils"
)

// ProviderResponse represents the provider schema returned from the provider registry
type ProviderResponse struct {
	ID                  string `json:"id"`
	ProviderName        string `json:"provider_name"`
	ProviderURL         string `json:"provider_url"`
	VerificationPending bool   `json:"verification_pending"`
	Version             string `json:"version"`
	VerifiedAt          *int64 `json:"verified_at,omitempty"`
	ProviderType        string `json:"provider_type"`
	CreatedAt           int64  `json:"created_at"`
}

// listVerifiedProviders fetches all verified providers from the provider registry service
func (s *Server) listVerifiedProviders(w http.ResponseWriter, r *http.Request) {
	providersURL := os.Getenv("PROVIDERS_SERVICE_URL")
	if providersURL == "" {
		providersURL = "http://localhost:8082"
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	reqURL := fmt.Sprintf("%s/api/providers", providersURL)
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, reqURL, nil)
	if err != nil {
		utils.LogHandlerError(r, "listVerifiedProviders", err, nil)
		utils.WriteError(w, http.StatusInternalServerError, "failed to create request")
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		utils.LogHandlerError(r, "listVerifiedProviders", err, map[string]any{"url": reqURL})
		utils.WriteError(w, http.StatusBadGateway, "failed to contact providers service")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("providers service returned status: %d", resp.StatusCode)
		utils.LogHandlerError(r, "listVerifiedProviders", err, map[string]any{"url": reqURL, "statusCode": resp.StatusCode})
		utils.WriteError(w, http.StatusBadGateway, "providers service returned non-200 response")
		return
	}

	var allProviders []ProviderResponse
	if err := json.NewDecoder(resp.Body).Decode(&allProviders); err != nil {
		utils.LogHandlerError(r, "listVerifiedProviders", err, map[string]any{"url": reqURL})
		utils.WriteError(w, http.StatusBadGateway, "failed to parse providers response")
		return
	}

	// Filter only verified providers
	verified := make([]ProviderResponse, 0)
	for _, p := range allProviders {
		if !p.VerificationPending {
			verified = append(verified, p)
		}
	}

	utils.WriteJSON(w, http.StatusOK, verified)
}

func (s *Server) reverseProxyProvider(w http.ResponseWriter, r *http.Request) {
	providerName := mux.Vars(r)["provider"]
	if providerName == "" {
		utils.WriteError(w, http.StatusBadRequest, "provider name missing")
		return
	}

	prov, err := s.providers.Get(r.Context(), providerName)
	if err != nil {
		utils.LogHandlerError(r, "reverseProxyProvider", err, map[string]any{"providerName": providerName})
		utils.WriteError(w, http.StatusBadGateway, "provider not found or pending verification")
		return
	}

	prov.Proxy.ServeHTTP(w, r)
}
