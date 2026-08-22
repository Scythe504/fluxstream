package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler(t *testing.T) {
	s := &Server{}
	server := httptest.NewServer(http.HandlerFunc(s.HelloWorldHandler))
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("error making request to server. Err: %v", err)
	}
	defer resp.Body.Close()
	// Assertions
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK; got %v", resp.Status)
	}
	expected := "{\"message\":\"Hello World\"}\n"
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading response body. Err: %v", err)
	}
	if expected != string(body) {
		t.Errorf("expected response body to be %v; got %v", expected, string(body))
	}
}

func TestListVerifiedProviders(t *testing.T) {
	// 1. Create a local mock server representing the providers microservice
	mockProvidersService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/providers" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Return two providers: one verified and one pending
		providers := []ProviderResponse{
			{
				ID:                  "prov-1",
				ProviderName:        "VerifiedAni",
				ProviderURL:         "http://localhost:8081",
				VerificationPending: false,
				Version:             "1.0.0",
				ProviderType:        "anime",
			},
			{
				ID:                  "prov-2",
				ProviderName:        "UnverifiedShow",
				ProviderURL:         "http://localhost:8089",
				VerificationPending: true,
				Version:             "1.0.0",
				ProviderType:        "series",
			},
		}
		_ = json.NewEncoder(w).Encode(providers)
	}))
	defer mockProvidersService.Close()

	// 2. Set the environment variable to mock server URL
	t.Setenv("PROVIDERS_SERVICE_URL", mockProvidersService.URL)

	s := &Server{}
	server := httptest.NewServer(http.HandlerFunc(s.listVerifiedProviders))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("error making request to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status OK, got %v", resp.Status)
	}

	var verifiedList []ProviderResponse
	if err := json.NewDecoder(resp.Body).Decode(&verifiedList); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// 3. Assertions: check that only the verified provider was returned
	if len(verifiedList) != 1 {
		t.Errorf("expected verified list length 1, got %d", len(verifiedList))
	} else if verifiedList[0].ProviderName != "VerifiedAni" {
		t.Errorf("expected verified provider name 'VerifiedAni', got '%s'", verifiedList[0].ProviderName)
	}
}
