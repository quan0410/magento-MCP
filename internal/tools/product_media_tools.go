package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerProductMediaTools(add toolAdder) {
	add(
		"magento_list_product_media",
		"List all media gallery entries and images for a product by SKU in Magento.",
		baseProps(map[string]interface{}{
			"sku": map[string]interface{}{"type": "string", "description": "Product SKU"},
		}),
		[]string{"sku"},
		handleMagentoListProductMedia,
	)

	add(
		"magento_get_product_media",
		"Retrieve a specific media gallery entry for a product by SKU and Entry ID in Magento.",
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
	return rawResult(data)
}
