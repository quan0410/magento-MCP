package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerCategoryTools(add toolAdder) {
	add(
		"magento_list_categories",
		"Retrieve the hierarchical category tree from Magento.",
		baseProps(map[string]interface{}{
			"root_category_id": map[string]interface{}{"type": "string", "description": "Optional parent category ID to start the tree from"},
			"depth":            map[string]interface{}{"type": "integer", "description": "Maximum tree depth to retrieve"},
		}),
		nil,
		handleMagentoListCategories,
	)

	add(
		"magento_get_category",
		"Retrieve detailed category information by Category ID.",
		baseProps(map[string]interface{}{
			"category_id": map[string]interface{}{"type": "string", "description": "Category ID"},
		}),
		[]string{"category_id"},
		handleMagentoGetCategory,
	)

	add(
		"magento_list_category_products",
		"List all product SKUs assigned to a specific category.",
		baseProps(map[string]interface{}{
			"category_id": map[string]interface{}{"type": "string", "description": "Category ID"},
		}),
		[]string{"category_id"},
		handleMagentoListCategoryProducts,
	)

	add(
		"magento_assign_category_product",
		"Assign a product SKU to a category with optional display position.",
		baseProps(map[string]interface{}{
			"category_id": map[string]interface{}{"type": "string", "description": "Category ID"},
			"sku":         map[string]interface{}{"type": "string", "description": "Product SKU"},
			"position":    map[string]interface{}{"type": "integer", "description": "Display sort position (default 0)"},
		}),
		[]string{"category_id", "sku"},
		handleMagentoAssignCategoryProduct,
	)
}

func handleMagentoListCategories(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	q := url.Values{}
	if rootID := strArg(m, "root_category_id"); rootID != "" {
		q.Set("rootCategoryId", rootID)
	}
	if depth := intArg(m, "depth", 0); depth > 0 {
		q.Set("depth", fmt.Sprintf("%d", depth))
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/categories", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetCategory(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	catID := strArg(m, "category_id")
	if catID == "" {
		return mcp.ToolResultError("category_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/categories/%s", catID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoListCategoryProducts(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	catID := strArg(m, "category_id")
	if catID == "" {
		return mcp.ToolResultError("category_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/categories/%s/products", catID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoAssignCategoryProduct(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	catID := strArg(m, "category_id")
	sku := strArg(m, "sku")
	if catID == "" || sku == "" {
		return mcp.ToolResultError("category_id and sku are required")
	}

	productLink := map[string]interface{}{
		"sku":         sku,
		"position":    intArg(m, "position", 0),
		"category_id": catID,
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPost, fmt.Sprintf("/categories/%s/products", catID), url.Values{}, map[string]interface{}{"productLink": productLink})
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
