package magento_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/datumbridge/magento-mcp/internal/magento"
)

func TestSignOAuth1KnownVector(t *testing.T) {
	params := map[string]string{
		"status":                 "Hello Ladies + Gentlemen, a signed OAuth request!",
		"include_entities":       "true",
		"oauth_consumer_key":     "xvz1evFS4wEEPTGEFPHBog",
		"oauth_nonce":            "kYjzVBB8Y0ZFabxSWbWovY3uYSQ2pTgmZeNu2VS4cg",
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        "1318622958",
		"oauth_token":            "370773112-GmHxMAgYyLbNEtIKZeRNFsMKPR9EyMZeS9weJAEb",
		"oauth_version":          "1.0",
	}
	consumerSecret := "kAcSOqF21Fu85e7zjz7ZN2U4ZRhfV3WpwPAoE3Z7kBw"
	tokenSecret := "LswwdoUaIvS8ltyTt5jkRh4J50vUPVVHtR2oPAEW"

	got := magento.SignOAuth1(http.MethodPost, "https://api.twitter.com/1/statuses/update.json", params, consumerSecret, tokenSecret)
	want := "jwn8zVcmo8/kJzlMS4Xct+KnZbU="
	if got != want {
		t.Fatalf("SignOAuth1() = %q, want %q", got, want)
	}
}

func TestBuildOAuth1Header(t *testing.T) {
	// Default: HMAC-SHA256
	h, err := magento.BuildOAuth1Header(
		http.MethodGet,
		"https://example.com/rest/V1/products",
		"ck_key",
		"cs_secret",
		"at_token",
		"ats_secret",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(h, "OAuth ") {
		t.Fatalf("header should start with OAuth, got %s", h)
	}
	for _, field := range []string{
		`oauth_consumer_key="ck_key"`,
		`oauth_token="at_token"`,
		`oauth_signature_method="HMAC-SHA256"`,
		`oauth_signature="`,
		`oauth_nonce="`,
		`oauth_timestamp="`,
		`oauth_version="1.0"`,
	} {
		if !strings.Contains(h, field) {
			t.Errorf("missing field %s in header: %s", field, h)
		}
	}

	// Explicit: HMAC-SHA1
	hSHA1, err := magento.BuildOAuth1HeaderWithMethod(
		http.MethodGet,
		"https://example.com/rest/V1/products",
		"ck_key",
		"cs_secret",
		"at_token",
		"ats_secret",
		magento.SignatureMethodSHA1,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(hSHA1, `oauth_signature_method="HMAC-SHA1"`) {
		t.Errorf("expected HMAC-SHA1 in header: %s", hSHA1)
	}
}
