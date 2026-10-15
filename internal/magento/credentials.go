package magento

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Credentials matches datumbridge-integrations' MagentoTokenBundle
// (internal/credentials/vault.go).
type Credentials struct {
	BaseURL           string `json:"base_url"`
	ConsumerKey       string `json:"consumer_key,omitempty"`
	ConsumerSecret    string `json:"consumer_secret,omitempty"`
	AccessToken       string `json:"access_token"`
	AccessTokenSecret string `json:"access_token_secret,omitempty"`
	AuthMode          string `json:"auth_mode,omitempty"`
}

// IsBearer returns true if Bearer token authentication should be used.
func (c *Credentials) IsBearer() bool {
	if c == nil {
		return false
	}
	mode := strings.ToLower(strings.TrimSpace(c.AuthMode))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(os.Getenv("MAGENTO_AUTH_MODE")))
	}
	if mode == "bearer" {
		return true
	}
	if mode == "oauth1" {
		return false
	}
	// If any OAuth 1.0a keys are missing, default to Bearer
	if c.ConsumerKey == "" || c.ConsumerSecret == "" || c.AccessTokenSecret == "" {
		return true
	}
	return false
}

// NormalizeBaseURL strips trailing slashes and ensures standard scheme.
func NormalizeBaseURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "https://" + s
	}
	return strings.TrimRight(s, "/")
}

// ParseCredentials loads Credentials from vault args (credentials_json /
// credentials_path), then falls back to process env for local/dev runs.
func ParseCredentials(credentialsJSON, credentialsPath string) (*Credentials, error) {
	c := &Credentials{}
	raw := strings.TrimSpace(credentialsJSON)

	if raw == "" && strings.TrimSpace(credentialsPath) != "" {
		path := filepath.Clean(credentialsPath)
		if strings.Contains(path, "..") {
			return nil, fmt.Errorf("credentials_path must not contain ..")
		}
		allowedRoot := strings.TrimSpace(os.Getenv("MAGENTO_CREDENTIALS_DIR"))
		if allowedRoot == "" {
			allowedRoot = "/credentials"
		}
		allowedRoot = filepath.Clean(allowedRoot)
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("credentials_path: %w", err)
		}
		absRoot, err := filepath.Abs(allowedRoot)
		if err != nil {
			return nil, fmt.Errorf("MAGENTO_CREDENTIALS_DIR: %w", err)
		}
		resolvedRoot, err := filepath.EvalSymlinks(absRoot)
		if err != nil {
			resolvedRoot = absRoot
		}
		resolvedPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return nil, fmt.Errorf("credentials_path: %w", err)
		}
		if resolvedPath != resolvedRoot && !strings.HasPrefix(resolvedPath, resolvedRoot+string(os.PathSeparator)) {
			return nil, fmt.Errorf("credentials_path must be under %s", absRoot)
		}
		b, err := os.ReadFile(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("read credentials_path: %w", err)
		}
		raw = strings.TrimSpace(string(b))
	}

	if raw != "" {
		if err := json.Unmarshal([]byte(raw), c); err != nil {
			return nil, fmt.Errorf("invalid credentials_json: %w", err)
		}
	}

	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = os.Getenv("MAGENTO_BASE_URL")
	}
	if strings.TrimSpace(c.ConsumerKey) == "" {
		c.ConsumerKey = os.Getenv("MAGENTO_CONSUMER_KEY")
	}
	if strings.TrimSpace(c.ConsumerSecret) == "" {
		c.ConsumerSecret = os.Getenv("MAGENTO_CONSUMER_SECRET")
	}
	if strings.TrimSpace(c.AccessToken) == "" {
		c.AccessToken = os.Getenv("MAGENTO_ACCESS_TOKEN")
	}
	if strings.TrimSpace(c.AccessTokenSecret) == "" {
		c.AccessTokenSecret = os.Getenv("MAGENTO_ACCESS_TOKEN_SECRET")
	}
	if strings.TrimSpace(c.AuthMode) == "" {
		c.AuthMode = os.Getenv("MAGENTO_AUTH_MODE")
	}

	c.BaseURL = NormalizeBaseURL(c.BaseURL)
	c.ConsumerKey = strings.TrimSpace(c.ConsumerKey)
	c.ConsumerSecret = strings.TrimSpace(c.ConsumerSecret)
	c.AccessToken = strings.TrimSpace(c.AccessToken)
	c.AccessTokenSecret = strings.TrimSpace(c.AccessTokenSecret)
	c.AuthMode = strings.TrimSpace(c.AuthMode)

	if c.BaseURL == "" {
		return nil, fmt.Errorf("base_url required (credentials_json.base_url or MAGENTO_BASE_URL)")
	}
	if c.AccessToken == "" {
		return nil, fmt.Errorf("access_token required (credentials_json.access_token or MAGENTO_ACCESS_TOKEN)")
	}
	if !c.IsBearer() {
		if c.ConsumerKey == "" || c.ConsumerSecret == "" || c.AccessTokenSecret == "" {
			return nil, fmt.Errorf("all 4 OAuth 1.0a keys required (consumer_key, consumer_secret, access_token, access_token_secret) or use bearer auth mode")
		}
	}
	return c, nil
}

// MaskKey returns a redacted preview for health/debug output.
func MaskKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return "(missing)"
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "…" + key[len(key)-4:]
}
