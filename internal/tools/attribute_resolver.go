package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datumbridge/magento-mcp/internal/magento"
)

type attributeMeta struct {
	id    int
	label string
}

// attributeOptionResolver dynamically resolves attribute option labels (e.g. "Red", "M", "One Size")
// to their corresponding Magento option IDs (e.g. 58, 168) for dropdown and multiselect attributes.
// If an option does not exist yet for a select attribute, it automatically creates it on the fly in Magento.
type attributeOptionResolver struct {
	client    *magento.Client
	mu        sync.RWMutex
	options   map[string]map[string]string // attributeCode -> map[lowercase_label_or_value]option_value_id
	noOptions map[string]bool              // attributeCode -> true if attribute is text/textarea/not found
	meta      map[string]attributeMeta     // attributeCode -> {id, label}
}

func newAttributeOptionResolver(client *magento.Client) *attributeOptionResolver {
	return &attributeOptionResolver{
		client:    client,
		options:   make(map[string]map[string]string),
		noOptions: make(map[string]bool),
		meta:      make(map[string]attributeMeta),
	}
}

// Resolve maps a raw value (e.g. string "Red" or "One Size") to the integer option ID or string ID.
// If the attribute is not a select attribute, it returns rawVal unmodified.
func (r *attributeOptionResolver) Resolve(ctx context.Context, attributeCode string, rawVal interface{}) interface{} {
	if rawVal == nil {
		return nil
	}

	strVal := strings.TrimSpace(fmt.Sprint(rawVal))
	if strVal == "" {
		return rawVal
	}

	code := strings.ToLower(strings.TrimSpace(attributeCode))

	r.mu.RLock()
	if r.noOptions[code] {
		r.mu.RUnlock()
		return rawVal
	}
	optMap, cached := r.options[code]
	r.mu.RUnlock()

	if !cached {
		r.mu.Lock()
		if r.noOptions[code] {
			r.mu.Unlock()
			return rawVal
		}
		optMap, cached = r.options[code]
		if !cached {
			optMap = r.fetchOptions(ctx, code)
			if optMap == nil || len(optMap) == 0 {
				r.noOptions[code] = true
			} else {
				r.options[code] = optMap
			}
		}
		r.mu.Unlock()
	}

	if optMap == nil {
		return rawVal
	}

	// 1. Check direct match (label or value)
	lowerVal := strings.ToLower(strVal)
	if valID, found := optMap[lowerVal]; found {
		if intVal, err := strconv.Atoi(valID); err == nil {
			return intVal
		}
		return valID
	}

	// 2. Check comma-separated multiselect options (e.g. "Red, Blue" or "58, 50")
	if strings.Contains(strVal, ",") {
		parts := strings.Split(strVal, ",")
		allResolved := true
		resolvedParts := make([]string, 0, len(parts))
		for _, p := range parts {
			pTrim := strings.ToLower(strings.TrimSpace(p))
			if pTrim == "" {
				continue
			}
			if valID, found := optMap[pTrim]; found {
				resolvedParts = append(resolvedParts, valID)
			} else {
				// Try creating missing option for multiselect part
				newPartID := r.createOption(ctx, code, strings.TrimSpace(p))
				if newPartID != "" {
					resolvedParts = append(resolvedParts, newPartID)
				} else {
					allResolved = false
					break
				}
			}
		}
		if allResolved && len(resolvedParts) > 0 {
			return strings.Join(resolvedParts, ",")
		}
	}

	// 3. Since this attribute IS a select attribute with options, but strVal is not in optMap,
	// automatically create the missing option on the fly in Magento!
	newID := r.createOption(ctx, code, strVal)
	if newID != "" {
		if intVal, err := strconv.Atoi(newID); err == nil {
			return intVal
		}
		return newID
	}

	return rawVal
}

func (r *attributeOptionResolver) fetchOptions(ctx context.Context, code string) map[string]string {
	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	path := "/products/attributes/" + url.PathEscape(code) + "/options"
	resp, status, err := r.client.Request(reqCtx, http.MethodGet, path, url.Values{}, nil)
	if err != nil || status != http.StatusOK {
		return nil
	}

	var opts []struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal(resp, &opts); err != nil || len(opts) == 0 {
		return nil
	}

	optMap := make(map[string]string)
	hasValidOption := false

	for _, opt := range opts {
		val := strings.TrimSpace(opt.Value)
		label := strings.ToLower(strings.TrimSpace(opt.Label))
		if val != "" {
			hasValidOption = true
			if label != "" {
				optMap[label] = val
			}
			// Map existing value to itself
			optMap[strings.ToLower(val)] = val
		}
	}

	if !hasValidOption {
		return nil
	}

	return optMap
}

func (r *attributeOptionResolver) createOption(ctx context.Context, code string, label string) string {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double check cache
	lowerLabel := strings.ToLower(strings.TrimSpace(label))
	if optMap, ok := r.options[code]; ok && optMap != nil {
		if valID, found := optMap[lowerLabel]; found {
			return valID
		}
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	payload := map[string]interface{}{
		"option": map[string]interface{}{
			"label":      label,
			"sort_order": 99,
			"is_default": false,
		},
	}
	path := "/products/attributes/" + url.PathEscape(code) + "/options"
	resp, status, err := r.client.Request(reqCtx, http.MethodPost, path, url.Values{}, payload)
	if err != nil || status != http.StatusOK {
		return ""
	}

	// In Magento 2, POST options often returns the new option ID as JSON string e.g. "212"
	var newID string
	if err := json.Unmarshal(resp, &newID); err == nil && newID != "" && newID != "true" {
		if r.options[code] == nil {
			r.options[code] = make(map[string]string)
		}
		r.options[code][lowerLabel] = newID
		r.options[code][strings.ToLower(newID)] = newID
		return newID
	}

	// Fallback: Re-fetch options from Magento to locate the newly created option ID
	refreshed := r.fetchOptions(ctx, code)
	if refreshed != nil {
		r.options[code] = refreshed
		if valID, found := refreshed[lowerLabel]; found {
			return valID
		}
	}

	return ""
}

// GetAttributeInfo returns the attribute_id and default frontend label for a given attribute code.
func (r *attributeOptionResolver) GetAttributeInfo(ctx context.Context, code string) (int, string, error) {
	code = strings.ToLower(strings.TrimSpace(code))
	r.mu.RLock()
	if m, ok := r.meta[code]; ok {
		r.mu.RUnlock()
		return m.id, m.label, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if m, ok := r.meta[code]; ok {
		return m.id, m.label, nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	path := "/products/attributes/" + url.PathEscape(code)
	resp, status, err := r.client.Request(reqCtx, http.MethodGet, path, url.Values{}, nil)
	if err != nil || status != http.StatusOK {
		return 0, "", fmt.Errorf("failed to fetch attribute '%s': status %d, err %v", code, status, err)
	}

	var attr struct {
		AttributeID   int    `json:"attribute_id"`
		FrontendLabel string `json:"default_frontend_label"`
	}
	if err := json.Unmarshal(resp, &attr); err != nil {
		return 0, "", fmt.Errorf("failed to decode attribute '%s': %w", code, err)
	}

	label := attr.FrontendLabel
	if label == "" {
		label = strings.Title(code)
	}
	r.meta[code] = attributeMeta{id: attr.AttributeID, label: label}
	return attr.AttributeID, label, nil
}

// ResolveOptionInt resolves a raw attribute value to its integer Magento option ID.
func (r *attributeOptionResolver) ResolveOptionInt(ctx context.Context, attributeCode string, rawVal interface{}) (int, error) {
	resolved := r.Resolve(ctx, attributeCode, rawVal)
	if resolved == nil {
		return 0, fmt.Errorf("nil value for attribute '%s'", attributeCode)
	}
	switch v := resolved.(type) {
	case int:
		return v, nil
	case float64:
		return int(v), nil
	case string:
		if id, err := strconv.Atoi(v); err == nil && id > 0 {
			return id, nil
		}
		return 0, fmt.Errorf("could not resolve '%s' to option ID for attribute '%s'", v, attributeCode)
	default:
		return 0, fmt.Errorf("unexpected resolved type %T for attribute '%s'", v, attributeCode)
	}
}
