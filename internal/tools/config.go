package tools

import (
	"os"
	"strings"
)

// ServerConfig captures runtime tool selection for magento-mcp.
type ServerConfig struct {
	Allowlist map[string]struct{}
}

// LoadServerConfig parses MAGENTO_TOOLS (comma-separated list of enabled tool names).
// Empty means all registered tools enabled.
func LoadServerConfig() *ServerConfig {
	raw := strings.TrimSpace(os.Getenv("MAGENTO_TOOLS"))
	if raw == "" {
		return &ServerConfig{}
	}
	parts := strings.Split(raw, ",")
	m := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			m[p] = struct{}{}
		}
	}
	return &ServerConfig{Allowlist: m}
}

func (c *ServerConfig) toolEnabled(name string) bool {
	if c == nil || len(c.Allowlist) == 0 {
		return true
	}
	_, ok := c.Allowlist[name]
	return ok
}
