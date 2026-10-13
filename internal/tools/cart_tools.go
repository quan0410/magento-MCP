package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerCartTools(add toolAdder) {
	add(
		"magento_create_empty_cart",
		"Create a new empty cart (quote) in Magento.",
		baseProps(nil),
		nil,
		handleMagentoCreateEmptyCart,
	)

	add(
		"magento_get_cart",
		"Retrieve full cart details including items, billing, and totals by Cart ID.",
		baseProps(map[string]interface{}{
			"cart_id": map[string]interface{}{"type": "string", "description": "Cart / Quote ID"},
		}),
		[]string{"cart_id"},
		handleMagentoGetCart,
	)

	add(
		"magento_add_cart_item",
		"Add a product item to a specific cart by Cart ID and SKU.",
		baseProps(map[string]interface{}{
			"cart_id": map[string]interface{}{"type": "string", "description": "Cart / Quote ID"},
			"sku":     map[string]interface{}{"type": "string", "description": "Product SKU"},
			"qty":     map[string]interface{}{"type": "number", "description": "Quantity to add (default 1)"},
		}),
		[]string{"cart_id", "sku"},
		handleMagentoAddCartItem,
	)
}

func handleMagentoCreateEmptyCart(args json.RawMessage) map[string]interface{} {
	client, _, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPost, "/carts/", url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetCart(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	cartID := strArg(m, "cart_id")
	if cartID == "" {
		return mcp.ToolResultError("cart_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/carts/%s", cartID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoAddCartItem(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	cartID := strArg(m, "cart_id")
	if cartID == "" {
		return mcp.ToolResultError("cart_id is required")
	}
	sku := strArg(m, "sku")
	if sku == "" {
		return mcp.ToolResultError("sku is required")
	}
	qty := floatArg(m, "qty", 1)

	body := map[string]interface{}{
		"cartItem": map[string]interface{}{
			"quote_id": cartID,
			"sku":      sku,
			"qty":      qty,
		},
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPost, fmt.Sprintf("/carts/%s/items", cartID), url.Values{}, body)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
