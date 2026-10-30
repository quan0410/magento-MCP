package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerInventoryTools(add toolAdder) {
	add(
		"magento_get_stock_item",
		"Retrieve stock item details (quantity, in_stock status, manage_stock flag) for a product SKU.",
		baseProps(map[string]interface{}{
			"sku": map[string]interface{}{"type": "string", "description": "Product SKU"},
		}),
		[]string{"sku"},
		handleMagentoGetStockItem,
	)

	add(
		"magento_update_stock_item",
		"Update product stock quantity and in_stock status for a product SKU.",
		baseProps(map[string]interface{}{
			"sku":         map[string]interface{}{"type": "string", "description": "Product SKU"},
			"item_id":     map[string]interface{}{"type": "string", "description": "Stock item entity ID"},
			"qty":         map[string]interface{}{"type": "number", "description": "Stock quantity"},
			"is_in_stock": map[string]interface{}{"type": "boolean", "description": "Whether item is in stock"},
		}),
		[]string{"sku", "item_id"},
		handleMagentoUpdateStockItem,
	)

	add(
		"magento_list_inventory_sources",
		"List inventory sources (Multi-Source Inventory / MSI) configured in Magento.",
		baseProps(nil),
		nil,
		handleMagentoListInventorySources,
	)

	add(
		"magento_get_source_items",
		"Retrieve inventory source items (Multi-Source Inventory / MSI) by product SKU and optional source_code, showing stock quantity and status per source/warehouse.",
		baseProps(map[string]interface{}{
			"sku":          map[string]interface{}{"type": "string", "description": "Product SKU to query inventory for"},
			"source_code":  map[string]interface{}{"type": "string", "description": "Optional inventory source code to filter by specific store/warehouse"},
			"page_size":    map[string]interface{}{"type": "integer", "description": "Number of items per page (default 50)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Page number (default 1)"},
		}),
		[]string{"sku"},
		handleMagentoGetSourceItems,
	)
}

func handleMagentoGetStockItem(args json.RawMessage) map[string]interface{} {
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
	data, _, err := client.Request(c, http.MethodGet, "/stockItems/"+url.PathEscape(sku), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoUpdateStockItem(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	sku := strArg(m, "sku")
	itemID := strArg(m, "item_id")
	if sku == "" || itemID == "" {
		return mcp.ToolResultError("sku and item_id are required")
	}

	stockItem := map[string]interface{}{}
	if q := floatArg(m, "qty", -1); q >= 0 {
		stockItem["qty"] = q
	}
	if inStock := boolArg(m, "is_in_stock", true); inStock {
		stockItem["is_in_stock"] = true
	} else {
		stockItem["is_in_stock"] = false
	}

	c, cancel := ctx()
	defer cancel()
	endpoint := fmt.Sprintf("/products/%s/stockItems/%s", url.PathEscape(sku), itemID)
	data, _, err := client.Request(c, http.MethodPut, endpoint, url.Values{}, map[string]interface{}{"stockItem": stockItem})
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoListInventorySources(args json.RawMessage) map[string]interface{} {
	client, _, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/inventory/sources", url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetSourceItems(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	sku := strArg(m, "sku")
	if sku == "" {
		return mcp.ToolResultError("sku is required")
	}
	pageSize := intArg(m, "page_size", 50)
	currentPage := intArg(m, "current_page", 1)

	q := url.Values{}
	fb := newFilterBuilder(q, pageSize, currentPage)
	fb.add("sku", sku, "eq")
	if sc := strArg(m, "source_code"); sc != "" {
		fb.add("source_code", sc, "eq")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/inventory/source-items", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
