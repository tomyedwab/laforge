package proxy

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
)

// AnthropicProxy handles proxying requests to Anthropic API
type AnthropicProxy struct {
	baseUrl    string
	apiKey     string
	oauthToken string
	proxy      *httputil.ReverseProxy
}

// NewAnthropicProxy creates a new Anthropic API proxy
// Accepts a base URL and either apiKey or oauthToken (preferring oauthToken if
// both are provided)
func NewAnthropicProxy(baseUrl, apiKey, oauthToken string) *AnthropicProxy {
	target, err := url.Parse(baseUrl)
	if err != nil {
		// This should never happen with a constant URL, but handle it defensively
		panic("failed to parse Anthropic API base URL: " + err.Error())
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// Customize the director to add authentication
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Use OAuth token if available, otherwise use API key
		if oauthToken != "" {
			// For OAuth, use Authorization: Bearer header
			req.Header.Set("Authorization", "Bearer "+oauthToken)
		} else if apiKey != "" {
			// For API key, use x-api-key header and strip any incoming Authorization header
			req.Header.Del("Authorization")
			req.Header.Set("x-api-key", apiKey)
		}

		// Only set anthropic-version if the client didn't send one
		if req.Header.Get("anthropic-version") == "" {
			req.Header.Set("anthropic-version", "2023-06-01")
		}
		req.Host = target.Host
	}

	return &AnthropicProxy{
		apiKey:     apiKey,
		oauthToken: oauthToken,
		proxy:      proxy,
	}
}

// ServeHTTP handles the proxy request
func (p *AnthropicProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	slog.Debug("proxying Anthropic API request",
		"method", r.Method,
		"path", r.URL.Path,
	)

	p.proxy.ServeHTTP(w, r)
}
