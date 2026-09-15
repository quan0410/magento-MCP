package magento

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Client calls Magento 2's REST API (/rest/V1/...), authenticated with the
// connecting merchant's OAuth 1.0a Integration key pair.
type Client struct {
	creds      *Credentials
	httpClient *http.Client
	apiBase    string // optional override for test suites
}

func NewClient(creds *Credentials) *Client {
	timeoutSec := 30
	if v := strings.TrimSpace(os.Getenv("MAGENTO_HTTP_TIMEOUT_SEC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
		}
	}
	base := strings.TrimSpace(os.Getenv("MAGENTO_API_BASE"))

	insecure := os.Getenv("MAGENTO_INSECURE_SKIP_VERIFY") == "true" || os.Getenv("MAGENTO_INSECURE_SKIP_VERIFY") == "1"
	if creds != nil && creds.BaseURL != "" {
		if u, err := url.Parse(creds.BaseURL); err == nil {
			host := u.Hostname()
			if strings.HasSuffix(host, ".test") || host == "localhost" || host == "127.0.0.1" {
				insecure = true
			}
		}
	}

	tr := &http.Transport{
		TLSClientConfig:    &tls.Config{InsecureSkipVerify: insecure},
		DisableCompression: true,
	}

	return &Client{
		creds:      creds,
		httpClient: &http.Client{Timeout: time.Duration(timeoutSec) * time.Second, Transport: tr},
		apiBase:    strings.TrimRight(base, "/"),
	}
}

func (c *Client) restBase() string {
	if c.apiBase != "" {
		return c.apiBase + "/rest/V1"
	}
	return c.creds.BaseURL + "/rest/V1"
}

// Request executes an OAuth 1.0a signed request against the Magento 2 REST API.
// It tries the configured or default signature method (HMAC-SHA256 for Magento 2.4.7+ / Mage-OS),
// and automatically falls back to HMAC-SHA1 if the store rejects the method.
func (c *Client) Request(ctx context.Context, method, path string, query url.Values, body interface{}) ([]byte, int, error) {
	primaryMethod := strings.TrimSpace(os.Getenv("MAGENTO_OAUTH_SIGNATURE_METHOD"))
	if primaryMethod == "" {
		primaryMethod = SignatureMethodSHA256
	}

	fallbackMethod := SignatureMethodSHA1
	if primaryMethod == SignatureMethodSHA1 {
		fallbackMethod = SignatureMethodSHA256
	}

	data, status, err := c.requestWithSignature(ctx, method, path, query, body, primaryMethod)
	if status == http.StatusBadRequest && strings.Contains(string(data), "Signature method") && strings.Contains(string(data), "is not supported") {
		return c.requestWithSignature(ctx, method, path, query, body, fallbackMethod)
	}
	return data, status, err
}

func (c *Client) requestWithSignature(ctx context.Context, method, path string, query url.Values, body interface{}, signatureMethod string) ([]byte, int, error) {
	if c == nil || c.creds == nil {
		return nil, 0, fmt.Errorf("magento client not configured")
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	full := c.restBase() + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	authHeader, err := BuildOAuth1HeaderWithMethod(
		method,
		full,
		c.creds.ConsumerKey,
		c.creds.ConsumerSecret,
		c.creds.AccessToken,
		c.creds.AccessTokenSecret,
		signatureMethod,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("build oauth1 header: %w", err)
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, full, reader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("magento request failed: %w", err)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, 10<<20) // 10 MiB cap
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := sanitizeProviderError(string(data))
		return data, resp.StatusCode, fmt.Errorf("magento HTTP %d: %s", resp.StatusCode, msg)
	}
	return data, resp.StatusCode, nil
}

// BaseURL returns the store's base URL.
func (c *Client) BaseURL() string {
	if c == nil || c.creds == nil {
		return ""
	}
	return c.creds.BaseURL
}

func sanitizeProviderError(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return "(empty body)"
	}
	if len(body) > 500 {
		body = body[:500] + "…"
	}
	lower := strings.ToLower(body)
	if strings.Contains(lower, "oauth_signature") || strings.Contains(lower, "consumer_secret") {
		return "(provider error sanitized)"
	}
	return body
}
