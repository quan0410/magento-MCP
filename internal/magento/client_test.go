package magento_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/datumbridge/magento-mcp/internal/magento"
)

func TestClient_RequestSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "OAuth ") || !strings.Contains(auth, `oauth_consumer_key="ck"`) {
			t.Fatalf("expected OAuth 1.0a Authorization header, got %q", auth)
		}
		if r.URL.Path != "/rest/V1/store/storeViews" {
			t.Fatalf("unexpected path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"code":"default","name":"Default Store View"}]`))
	}))
	defer ts.Close()

	t.Setenv("MAGENTO_API_BASE", ts.URL)
	creds := &magento.Credentials{
		BaseURL:           ts.URL,
		ConsumerKey:       "ck",
		ConsumerSecret:    "cs",
		AccessToken:       "at",
		AccessTokenSecret: "ats",
	}
	client := magento.NewClient(creds)

	data, status, err := client.Request(context.Background(), http.MethodGet, "/store/storeViews", url.Values{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if !strings.Contains(string(data), "Default Store View") {
		t.Fatalf("unexpected data: %s", string(data))
	}
}

func TestClient_RequestUnauthorized(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"The consumer isn't authorized to access %resources."}`))
	}))
	defer ts.Close()

	t.Setenv("MAGENTO_API_BASE", ts.URL)
	creds := &magento.Credentials{
		BaseURL:           ts.URL,
		ConsumerKey:       "ck_invalid",
		ConsumerSecret:    "cs",
		AccessToken:       "at",
		AccessTokenSecret: "ats",
	}
	client := magento.NewClient(creds)

	_, status, err := client.Request(context.Background(), http.MethodGet, "/store/storeViews", url.Values{}, nil)
	if err == nil {
		t.Fatal("expected error on 401 response")
	}
	if status != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", status)
	}
}
