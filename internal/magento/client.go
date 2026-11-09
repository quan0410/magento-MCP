package magento

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

// Client calls Magento 2's REST API (/rest/V1/...), authenticated with the
// connecting merchant's OAuth 1.0a Integration key pair or Bearer token.
// It uses tls-client to mimic a modern browser (Chrome JA3/JA4 TLS fingerprint)
// to bypass Cloudflare Bot Management / WAF challenges seamlessly.
type Client struct {
	creds      *Credentials
	httpClient tls_client.HttpClient
	apiBase    string // optional override for test suites
}

func NewClient(creds *Credentials) *Client {
	timeoutSec := 60
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

	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(timeoutSec),
		tls_client.WithClientProfile(profiles.Chrome_131),
		tls_client.WithNotFollowRedirects(),
	}
	if insecure {
		options = append(options, tls_client.WithInsecureSkipVerify())
	}

	httpClient, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		// Fallback to default client options if custom profile init fails
		httpClient, _ = tls_client.NewHttpClient(tls_client.NewNoopLogger())
	}

	return &Client{
		creds:      creds,
		httpClient: httpClient,
		apiBase:    strings.TrimRight(base, "/"),
	}
}

func (c *Client) restBase() string {
	if c.apiBase != "" {
		return c.apiBase + "/rest/V1"
	}
	return c.creds.BaseURL + "/rest/V1"
}

// encodeRFC3986 encodes url.Values strictly adhering to RFC 3986 (spaces as %20, unreserved chars unchanged).
func encodeRFC3986(v url.Values) string {
	if len(v) == 0 {
		return ""
	}
	var buf strings.Builder
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vs := v[k]
		kEsc := oauthEscape(k)
		for _, val := range vs {
			if buf.Len() > 0 {
				buf.WriteByte('&')
			}
			buf.WriteString(kEsc)
			buf.WriteByte('=')
			buf.WriteString(oauthEscape(val))
		}
	}
	return buf.String()
}

// applyBrowserHeaders injects browser-grade headers and custom bypass headers
// to ensure Cloudflare WAF treats the request as a legitimate browser session.
func (c *Client) applyBrowserHeaders(req *fhttp.Request) {
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9,vi;q=0.8")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Sec-Ch-Ua", `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	// Custom secret header for store-level Cloudflare WAF bypass (if configured)
	if c != nil && c.creds != nil && c.creds.CustomHeaderName != "" && c.creds.CustomHeaderValue != "" {
		req.Header.Set(c.creds.CustomHeaderName, c.creds.CustomHeaderValue)
	}
}

// Request executes a request against the Magento 2 REST API.
// If the credentials specify Bearer mode (or only access_token is configured), it uses Bearer authentication.
// Otherwise, it falls back to OAuth 1.0a (HMAC-SHA256 with HMAC-SHA1 fallback).
func (c *Client) Request(ctx context.Context, method, path string, query url.Values, body interface{}) ([]byte, int, error) {
	if c == nil || c.creds == nil {
		return nil, 0, fmt.Errorf("magento client not configured")
	}

	if c.creds.IsBearer() {
		return c.requestWithBearer(ctx, method, path, query, body)
	}

	primaryMethod := strings.TrimSpace(os.Getenv("MAGENTO_OAUTH_SIGNATURE_METHOD"))
	if primaryMethod == "" {
		primaryMethod = SignatureMethodSHA256
	}

	fallbackMethod := SignatureMethodSHA1
	if primaryMethod == SignatureMethodSHA1 {
		fallbackMethod = SignatureMethodSHA256
	}

	data, status, err := c.requestWithSignature(ctx, method, path, query, body, primaryMethod)
	if status == fhttp.StatusBadRequest && strings.Contains(string(data), "Signature method") && strings.Contains(string(data), "is not supported") {
		return c.requestWithSignature(ctx, method, path, query, body, fallbackMethod)
	}
	return data, status, err
}

func (c *Client) requestWithBearer(ctx context.Context, method, path string, query url.Values, body interface{}) ([]byte, int, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	full := c.restBase() + path
	if len(query) > 0 {
		full += "?" + encodeRFC3986(query)
	}

	reader, err := prepareRequestBody(body)
	if err != nil {
		return nil, 0, err
	}

	req, err := fhttp.NewRequestWithContext(ctx, method, full, reader)
	if err != nil {
		return nil, 0, err
	}

	c.applyBrowserHeaders(req)
	req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.doRequest(req)
}

func (c *Client) requestWithSignature(ctx context.Context, method, path string, query url.Values, body interface{}, signatureMethod string) ([]byte, int, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	full := c.restBase() + path
	if len(query) > 0 {
		full += "?" + encodeRFC3986(query)
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

	reader, err := prepareRequestBody(body)
	if err != nil {
		return nil, 0, err
	}

	req, err := fhttp.NewRequestWithContext(ctx, method, full, reader)
	if err != nil {
		return nil, 0, err
	}

	c.applyBrowserHeaders(req)
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.doRequest(req)
}

func prepareRequestBody(body interface{}) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	switch b := body.(type) {
	case []byte:
		return bytes.NewReader(b), nil
	case string:
		return strings.NewReader(b), nil
	case io.Reader:
		return b, nil
	default:
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("encode request body: %w", err)
		}
		return bytes.NewReader(payload), nil
	}
}

func (c *Client) doRequest(req *fhttp.Request) ([]byte, int, error) {
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

	// Check if blocked by Cloudflare challenge / WAF
	if resp.StatusCode == fhttp.StatusForbidden || resp.StatusCode == fhttp.StatusServiceUnavailable {
		lowerData := strings.ToLower(string(data))
		if strings.Contains(lowerData, "just a moment...") ||
			strings.Contains(lowerData, "challenges.cloudflare.com") ||
			strings.Contains(lowerData, "cf-mitigated") ||
			resp.Header.Get("cf-mitigated") == "challenge" {
			return data, resp.StatusCode, fmt.Errorf("magento HTTP %d: Cloudflare challenge/WAF blocked request (Bot Fight Mode active). Please configure WAF skip rule or verify merchant store URL", resp.StatusCode)
		}
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

// GraphQL executes a GraphQL query against Magento 2's /graphql endpoint.
func (c *Client) GraphQL(ctx context.Context, query string, variables map[string]interface{}) ([]byte, error) {
	if c == nil || c.creds == nil {
		return nil, fmt.Errorf("magento client not configured")
	}
	gqlURL := strings.TrimRight(c.creds.BaseURL, "/") + "/graphql"
	if c.apiBase != "" {
		gqlURL = strings.TrimRight(c.apiBase, "/") + "/graphql"
	}

	payload := map[string]interface{}{
		"query": query,
	}
	if variables != nil {
		payload["variables"] = variables
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode graphql payload: %w", err)
	}

	req, err := fhttp.NewRequestWithContext(ctx, fhttp.MethodPost, gqlURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create graphql request: %w", err)
	}
	c.applyBrowserHeaders(req)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if c.creds.IsBearer() && c.creds.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.creds.AccessToken)
	}

	data, status, err := c.doRequest(req)
	// If 401 or token error on GraphQL catalog request, fallback to public query without Authorization header
	if (status == fhttp.StatusUnauthorized || err != nil) && req.Header.Get("Authorization") != "" {
		reqPublic, errPub := fhttp.NewRequestWithContext(ctx, fhttp.MethodPost, gqlURL, bytes.NewReader(bodyBytes))
		if errPub == nil {
			c.applyBrowserHeaders(reqPublic)
			reqPublic.Header.Set("Content-Type", "application/json")
			reqPublic.Header.Set("Accept", "application/json")
			if pubData, pubStatus, pubErr := c.doRequest(reqPublic); pubErr == nil && pubStatus >= 200 && pubStatus < 300 {
				return pubData, nil
			}
		}
	}
	if err != nil {
		return data, err
	}
	if status < 200 || status >= 300 {
		return data, fmt.Errorf("graphql HTTP %d: %s", status, string(data))
	}
	return data, nil
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
