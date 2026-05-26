package provider

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

const ProviderUrl = "http://localhost:8081"

type Provider struct {
	Name         string
	BaseUrl      *url.URL
	MediaSupport []MediaType
	Proxy        *httputil.ReverseProxy
}

func InitProvider(name string, baseURL string) *Provider {
	targetURL, _ := url.Parse(baseURL)

	proxy := &httputil.ReverseProxy{
		Transport: &http.Transport{
			ResponseHeaderTimeout: 15 * time.Second,
		},
	}

	return &Provider{
		Name:    name,
		BaseUrl: targetURL,
		MediaSupport: []MediaType{
			MediaTypeAnime,
			MediaTypeMovie,
		},
		Proxy: proxy,
	}
}
