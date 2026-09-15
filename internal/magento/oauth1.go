package magento

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	SignatureMethodSHA1   = "HMAC-SHA1"
	SignatureMethodSHA256 = "HMAC-SHA256"
)

// BuildOAuth1Header builds a one-legged OAuth 1.0a Authorization header.
// If MAGENTO_OAUTH_SIGNATURE_METHOD is set, it uses that method; otherwise defaults to HMAC-SHA256
// (required by Magento 2.4.7+ / Mage-OS).
func BuildOAuth1Header(method, rawURL, consumerKey, consumerSecret, accessToken, accessTokenSecret string) (string, error) {
	sigMethod := os.Getenv("MAGENTO_OAUTH_SIGNATURE_METHOD")
	if sigMethod == "" {
		sigMethod = SignatureMethodSHA256
	}
	return BuildOAuth1HeaderWithMethod(method, rawURL, consumerKey, consumerSecret, accessToken, accessTokenSecret, sigMethod)
}

// BuildOAuth1HeaderWithMethod builds a one-legged OAuth 1.0a Authorization header with an explicit signature method.
func BuildOAuth1HeaderWithMethod(method, rawURL, consumerKey, consumerSecret, accessToken, accessTokenSecret, signatureMethod string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	nonce, err := generateNonce()
	if err != nil {
		return "", err
	}

	if signatureMethod == "" {
		signatureMethod = SignatureMethodSHA256
	}

	params := map[string]string{
		"oauth_consumer_key":     consumerKey,
		"oauth_nonce":            nonce,
		"oauth_signature_method": signatureMethod,
		"oauth_timestamp":        strconv.FormatInt(time.Now().Unix(), 10),
		"oauth_token":            accessToken,
		"oauth_version":          "1.0",
	}

	for k, vs := range u.Query() {
		for _, v := range vs {
			params[k] = v
		}
	}

	baseURL := u.Scheme + "://" + u.Host + u.Path
	params["oauth_signature"] = SignOAuth1WithMethod(method, baseURL, params, consumerSecret, accessTokenSecret, signatureMethod)

	oauthKeys := make([]string, 0, 7)
	for k := range params {
		if strings.HasPrefix(k, "oauth_") {
			oauthKeys = append(oauthKeys, k)
		}
	}
	sort.Strings(oauthKeys)
	headerParts := make([]string, 0, len(oauthKeys))
	for _, k := range oauthKeys {
		headerParts = append(headerParts, fmt.Sprintf(`%s="%s"`, oauthEscape(k), oauthEscape(params[k])))
	}
	return "OAuth " + strings.Join(headerParts, ", "), nil
}

func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// SignOAuth1 computes the OAuth 1.0a signature using HMAC-SHA1 for backward compatibility.
func SignOAuth1(method, baseURL string, params map[string]string, consumerSecret, tokenSecret string) string {
	return SignOAuth1WithMethod(method, baseURL, params, consumerSecret, tokenSecret, SignatureMethodSHA1)
}

// SignOAuth1WithMethod computes the OAuth 1.0a signature base string and HMAC signature
// using the specified method (HMAC-SHA256 or HMAC-SHA1).
func SignOAuth1WithMethod(method, baseURL string, params map[string]string, consumerSecret, tokenSecret, signatureMethod string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, oauthEscape(k)+"="+oauthEscape(params[k]))
	}
	paramString := strings.Join(pairs, "&")
	baseString := strings.ToUpper(method) + "&" + oauthEscape(baseURL) + "&" + oauthEscape(paramString)
	signingKey := oauthEscape(consumerSecret) + "&" + oauthEscape(tokenSecret)

	var mac hash.Hash
	if signatureMethod == SignatureMethodSHA256 {
		mac = hmac.New(sha256.New, []byte(signingKey))
	} else {
		mac = hmac.New(sha1.New, []byte(signingKey))
	}
	mac.Write([]byte(baseString))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// oauthEscape percent-encodes per RFC 3986 (unreserved: A-Za-z0-9-._~ only).
func oauthEscape(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
