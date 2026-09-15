package magento_test

import (
	"os"
	"testing"

	"github.com/datumbridge/magento-mcp/internal/magento"
)

func TestParseCredentials_InlineJSON(t *testing.T) {
	raw := `{
		"base_url": "https://mystore.example.com",
		"consumer_key": "ck_123",
		"consumer_secret": "cs_456",
		"access_token": "at_789",
		"access_token_secret": "ats_012"
	}`
	creds, err := magento.ParseCredentials(raw, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.BaseURL != "https://mystore.example.com" {
		t.Fatalf("got base_url %q, want https://mystore.example.com", creds.BaseURL)
	}
	if creds.ConsumerKey != "ck_123" || creds.ConsumerSecret != "cs_456" || creds.AccessToken != "at_789" || creds.AccessTokenSecret != "ats_012" {
		t.Fatalf("credentials fields do not match expected: %+v", creds)
	}
}

func TestParseCredentials_EnvFallback(t *testing.T) {
	t.Setenv("MAGENTO_BASE_URL", "https://env.example.com")
	t.Setenv("MAGENTO_CONSUMER_KEY", "env_ck")
	t.Setenv("MAGENTO_CONSUMER_SECRET", "env_cs")
	t.Setenv("MAGENTO_ACCESS_TOKEN", "env_at")
	t.Setenv("MAGENTO_ACCESS_TOKEN_SECRET", "env_ats")

	creds, err := magento.ParseCredentials("", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.BaseURL != "https://env.example.com" {
		t.Fatalf("got base_url %q, want https://env.example.com", creds.BaseURL)
	}
	if creds.ConsumerKey != "env_ck" {
		t.Fatalf("got consumer_key %q, want env_ck", creds.ConsumerKey)
	}
}

func TestParseCredentials_MissingRequired(t *testing.T) {
	_ = os.Unsetenv("MAGENTO_BASE_URL")
	_ = os.Unsetenv("MAGENTO_CONSUMER_KEY")
	_ = os.Unsetenv("MAGENTO_CONSUMER_SECRET")
	_ = os.Unsetenv("MAGENTO_ACCESS_TOKEN")
	_ = os.Unsetenv("MAGENTO_ACCESS_TOKEN_SECRET")

	if _, err := magento.ParseCredentials(`{"consumer_key":"ck"}`, ""); err == nil {
		t.Fatal("expected error when base_url missing")
	}
	if _, err := magento.ParseCredentials(`{"base_url":"https://example.com","consumer_key":"ck"}`, ""); err == nil {
		t.Fatal("expected error when keys missing")
	}
}
