package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/datumbridge/magento-mcp/internal/magento"
	"github.com/datumbridge/magento-mcp/internal/mcp"
)

// getBaseMediaURL returns the base media URL for Magento catalog products.
// It checks MAGENTO_MEDIA_URL env, or falls back to <baseURL>/media.
func getBaseMediaURL(client *magento.Client) string {
	if custom := strings.TrimSpace(os.Getenv("MAGENTO_MEDIA_URL")); custom != "" {
		return strings.TrimRight(custom, "/")
	}
	if client == nil {
		return ""
	}
	baseURL := strings.TrimRight(client.BaseURL(), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/media"
}

// formatProductMediaURL constructs a complete public URL for a product media file.
func formatProductMediaURL(mediaBase, rawFile string) string {
	rawFile = strings.TrimSpace(rawFile)
	if rawFile == "" {
		return ""
	}
	if strings.HasPrefix(rawFile, "http://") || strings.HasPrefix(rawFile, "https://") {
		return rawFile
	}
	cleanFile := strings.TrimPrefix(rawFile, "/")
	if mediaBase == "" {
		return "/" + cleanFile
	}
	mediaBase = strings.TrimRight(mediaBase, "/")
	if strings.HasPrefix(cleanFile, "catalog/product/") {
		return fmt.Sprintf("%s/%s", mediaBase, cleanFile)
	}
	return fmt.Sprintf("%s/catalog/product/%s", mediaBase, cleanFile)
}

// enrichMediaEntry adds complete image URLs to a media gallery entry map.
func enrichMediaEntry(item map[string]interface{}, mediaBase string) {
	if item == nil {
		return
	}
	fileStr, _ := item["file"].(string)
	if fileStr == "" {
		return
	}
	fullURL := formatProductMediaURL(mediaBase, fileStr)
	if fullURL == "" {
		return
	}
	// Preserve the original relative path in file_path
	item["file_path"] = fileStr
	// Provide complete URL in file, url, and full_url for maximum compatibility with UI tables and AI agents
	item["file"] = fullURL
	item["url"] = fullURL
	item["full_url"] = fullURL
}

func enrichImageAttribute(attr map[string]interface{}, mediaBase string, mainImageURL *string) {
	code, _ := attr["attribute_code"].(string)
	val, _ := attr["value"].(string)
	if val != "" && val != "no_selection" {
		switch code {
		case "image", "small_image", "thumbnail", "swatch_image":
			fullURL := formatProductMediaURL(mediaBase, val)
			attr["value_path"] = val
			attr["value"] = fullURL
			if code == "image" && *mainImageURL == "" {
				*mainImageURL = fullURL
			}
		}
	}
}

// enrichProductMedia enriches a Magento product object with complete image URLs
// in media_gallery_entries, custom_attributes (image, small_image, thumbnail, swatch_image),
// and provides a top-level image_url field.
func enrichProductMedia(prod map[string]interface{}, mediaBase string) {
	if prod == nil {
		return
	}

	// 1. Enrich media_gallery_entries
	if rawEntries, ok := prod["media_gallery_entries"].([]interface{}); ok {
		for _, re := range rawEntries {
			if entry, ok := re.(map[string]interface{}); ok {
				enrichMediaEntry(entry, mediaBase)
			}
		}
	} else if rawEntries, ok := prod["media_gallery_entries"].([]map[string]interface{}); ok {
		for i := range rawEntries {
			enrichMediaEntry(rawEntries[i], mediaBase)
		}
	}

	// 2. Enrich custom_attributes
	var mainImageURL string
	if rawCA, ok := prod["custom_attributes"].([]interface{}); ok {
		for _, item := range rawCA {
			if attr, ok := item.(map[string]interface{}); ok {
				enrichImageAttribute(attr, mediaBase, &mainImageURL)
			}
		}
	} else if rawCA, ok := prod["custom_attributes"].([]map[string]interface{}); ok {
		for i := range rawCA {
			enrichImageAttribute(rawCA[i], mediaBase, &mainImageURL)
		}
	}

	// 3. Convenient top-level image_url field
	if mainImageURL != "" {
		prod["image_url"] = mainImageURL
	} else if rawEntries, ok := prod["media_gallery_entries"].([]interface{}); ok && len(rawEntries) > 0 {
		if first, ok := rawEntries[0].(map[string]interface{}); ok {
			if u, ok := first["url"].(string); ok && u != "" {
				prod["image_url"] = u
			}
		}
	}
}

func registerProductMediaTools(add toolAdder) {
	add(
		"magento_list_product_media",
		"List all media gallery entries and images for a product by SKU in Magento, including complete image URLs.",
		baseProps(map[string]interface{}{
			"sku": map[string]interface{}{"type": "string", "description": "Product SKU"},
		}),
		[]string{"sku"},
		handleMagentoListProductMedia,
	)

	add(
		"magento_get_product_media",
		"Retrieve a specific media gallery entry for a product by SKU and Entry ID in Magento, including complete image URL.",
		baseProps(map[string]interface{}{
			"sku":      map[string]interface{}{"type": "string", "description": "Product SKU"},
			"entry_id": map[string]interface{}{"type": "string", "description": "Media entry ID"},
		}),
		[]string{"sku", "entry_id"},
		handleMagentoGetProductMedia,
	)
}

func handleMagentoListProductMedia(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	sku := strArg(m, "sku")
	if sku == "" {
		return mcp.ToolResultError("sku is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/products/%s/media", url.PathEscape(sku)), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	mediaBase := getBaseMediaURL(client)

	var entries []map[string]interface{}
	if err := json.Unmarshal(data, &entries); err == nil {
		for i := range entries {
			enrichMediaEntry(entries[i], mediaBase)
		}
		return jsonResult(entries)
	}

	return rawResult(data)
}

func handleMagentoGetProductMedia(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	sku := strArg(m, "sku")
	if sku == "" {
		return mcp.ToolResultError("sku is required")
	}
	entryID := strArg(m, "entry_id")
	if entryID == "" {
		return mcp.ToolResultError("entry_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/products/%s/media/%s", url.PathEscape(sku), entryID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	mediaBase := getBaseMediaURL(client)

	var entry map[string]interface{}
	if err := json.Unmarshal(data, &entry); err == nil {
		enrichMediaEntry(entry, mediaBase)
		return jsonResult(entry)
	}

	return rawResult(data)
}

