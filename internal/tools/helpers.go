package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/datumbridge/magento-mcp/internal/magento"
	"github.com/datumbridge/magento-mcp/internal/mcp"
)

// toolAdder registers one tool's descriptor + handler with the registry.
type toolAdder func(name, desc string, props map[string]interface{}, required []string, h mcp.ToolHandler)

func baseProps(extra map[string]interface{}) map[string]interface{} {
	props := map[string]interface{}{
		"credentials_json": map[string]interface{}{
			"type":        "string",
			"description": "Vault JSON: {base_url, consumer_key, consumer_secret, access_token, access_token_secret}",
		},
		"credentials_path": map[string]interface{}{
			"type":        "string",
			"description": "Optional path under MAGENTO_CREDENTIALS_DIR",
		},
	}
	for k, v := range extra {
		props[k] = v
	}
	return props
}

func schema(props map[string]interface{}, required []string) map[string]interface{} {
	s := map[string]interface{}{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func clientFrom(raw json.RawMessage) (*magento.Client, map[string]interface{}, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		m = map[string]interface{}{}
	}
	credsJSON, _ := m["credentials_json"].(string)
	credsPath, _ := m["credentials_path"].(string)
	creds, err := magento.ParseCredentials(credsJSON, credsPath)
	if err != nil {
		return nil, m, err
	}
	return magento.NewClient(creds), m, nil
}

func strArg(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func intArg(m map[string]interface{}, key string, def int) int {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case json.Number:
		i, _ := t.Int64()
		return int(i)
	case string:
		var i int
		if _, err := fmt.Sscanf(t, "%d", &i); err == nil {
			return i
		}
	}
	return def
}

func floatArg(m map[string]interface{}, key string, def float64) float64 {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	case json.Number:
		f, _ := t.Float64()
		return f
	case string:
		var f float64
		if _, err := fmt.Sscanf(t, "%f", &f); err == nil {
			return f
		}
	}
	return def
}

func boolArg(m map[string]interface{}, key string, def bool) bool {
	v, ok := m[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes"
	}
	return def
}

func mapArg(m map[string]interface{}, key string) map[string]interface{} {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	obj, _ := v.(map[string]interface{})
	return obj
}

func sliceMapArg(m map[string]interface{}, key string) []map[string]interface{} {
	v, ok := m[key]
	if !ok || v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	var out []map[string]interface{}
	for _, item := range arr {
		if obj, ok := item.(map[string]interface{}); ok {
			out = append(out, obj)
		}
	}
	return out
}

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 30*time.Second)
}

func jsonResult(v interface{}) map[string]interface{} {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return mcp.ToolResultText(string(b))
}

func rawResult(b []byte) map[string]interface{} {
	var v interface{}
	if err := json.Unmarshal(b, &v); err == nil {
		if pretty, err := json.MarshalIndent(v, "", "  "); err == nil {
			return mcp.ToolResultText(string(pretty))
		}
	}
	return mcp.ToolResultText(string(b))
}

type filterBuilder struct {
	q          url.Values
	groupCount int
}

func newFilterBuilder(q url.Values, pageSize, currentPage int) *filterBuilder {
	q.Set("searchCriteria[pageSize]", strconv.Itoa(pageSize))
	q.Set("searchCriteria[currentPage]", strconv.Itoa(currentPage))
	return &filterBuilder{q: q, groupCount: 0}
}

func (fb *filterBuilder) add(field, value, conditionType string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	if conditionType == "" {
		conditionType = "eq"
	}
	fb.q.Set(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][field]", fb.groupCount), field)
	fb.q.Set(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][value]", fb.groupCount), value)
	fb.q.Set(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][conditionType]", fb.groupCount), conditionType)
	fb.groupCount++
}

