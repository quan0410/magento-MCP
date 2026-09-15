package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerProductTools(add toolAdder) {
	add(
		"magento_list_products",
		"Search and list products in Magento 2 using searchCriteria (pageSize, currentPage, optional search name or category filter).",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Number of products per page (default 20, max 100)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Current page number (default 1)"},
			"name":         map[string]interface{}{"type": "string", "description": "Filter products whose name contains this value"},
			"type_id":      map[string]interface{}{"type": "string", "description": "Filter by product type (e.g. simple, configurable, virtual)"},
		}),
		nil,
		handleMagentoListProducts,
	)

	add(
		"magento_get_product",
		"Retrieve full product details by SKU.",
		baseProps(map[string]interface{}{
			"sku": map[string]interface{}{"type": "string", "description": "Product SKU"},
		}),
		[]string{"sku"},
		handleMagentoGetProduct,
	)

	add(
		"magento_create_product",
		"Create a new product in Magento with SKU, name, price, status, type_id, and attribute_set_id.",
		baseProps(map[string]interface{}{
			"sku":              map[string]interface{}{"type": "string", "description": "Unique product SKU"},
			"name":             map[string]interface{}{"type": "string", "description": "Product display name"},
			"price":            map[string]interface{}{"type": "number", "description": "Product unit price"},
			"status":           map[string]interface{}{"type": "integer", "description": "1 for Enabled, 2 for Disabled (default 1)"},
			"visibility":       map[string]interface{}{"type": "integer", "description": "1: Not visible, 2: Catalog, 3: Search, 4: Catalog & Search (default 4)"},
			"type_id":          map[string]interface{}{"type": "string", "description": "Product type (default simple)"},
			"attribute_set_id": map[string]interface{}{"type": "integer", "description": "Attribute set ID (default 4 for Default)"},
			"weight":           map[string]interface{}{"type": "number", "description": "Product weight"},
		}),
		[]string{"sku", "name", "price"},
		handleMagentoCreateProduct,
	)

	add(
		"magento_update_product",
		"Update an existing product's fields (name, price, status, visibility, weight) by SKU.",
		baseProps(map[string]interface{}{
			"sku":        map[string]interface{}{"type": "string", "description": "Product SKU to update"},
			"name":       map[string]interface{}{"type": "string", "description": "Updated name"},
			"price":      map[string]interface{}{"type": "number", "description": "Updated price"},
			"status":     map[string]interface{}{"type": "integer", "description": "1 for Enabled, 2 for Disabled"},
			"visibility": map[string]interface{}{"type": "integer", "description": "Visibility ID"},
			"weight":     map[string]interface{}{"type": "number", "description": "Updated weight"},
		}),
		[]string{"sku"},
		handleMagentoUpdateProduct,
	)

	add(
		"magento_delete_product",
		"Delete a product by SKU.",
		baseProps(map[string]interface{}{
			"sku": map[string]interface{}{"type": "string", "description": "Product SKU to delete"},
		}),
		[]string{"sku"},
		handleMagentoDeleteProduct,
	)

	add(
		"magento_list_product_types",
		"List all available product types supported by the Magento store.",
		baseProps(nil),
		nil,
		handleMagentoListProductTypes,
	)
}

func handleMagentoListProducts(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	pageSize := intArg(m, "page_size", 20)
	currentPage := intArg(m, "current_page", 1)

	q := url.Values{}
	q.Set("searchCriteria[pageSize]", strconv.Itoa(pageSize))
	q.Set("searchCriteria[currentPage]", strconv.Itoa(currentPage))

	filterIdx := 0
	if name := strArg(m, "name"); name != "" {
		q.Set(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][field]", filterIdx), "name")
		q.Set(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][value]", filterIdx), "%"+name+"%")
		q.Set(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][conditionType]", filterIdx), "like")
		filterIdx++
	}
	if typeID := strArg(m, "type_id"); typeID != "" {
		q.Set(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][field]", filterIdx), "type_id")
		q.Set(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][value]", filterIdx), typeID)
		q.Set(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][conditionType]", filterIdx), "eq")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/products", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetProduct(args json.RawMessage) map[string]interface{} {
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
	data, _, err := client.Request(c, http.MethodGet, "/products/"+url.PathEscape(sku), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoCreateProduct(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	sku := strArg(m, "sku")
	name := strArg(m, "name")
	if sku == "" || name == "" {
		return mcp.ToolResultError("sku and name are required")
	}

	price := floatArg(m, "price", 0.0)
	status := intArg(m, "status", 1)
	visibility := intArg(m, "visibility", 4)
	typeID := strArg(m, "type_id")
	if typeID == "" {
		typeID = "simple"
	}
	attrSetID := intArg(m, "attribute_set_id", 4)

	product := map[string]interface{}{
		"sku":              sku,
		"name":             name,
		"price":            price,
		"status":           status,
		"visibility":       visibility,
		"type_id":          typeID,
		"attribute_set_id": attrSetID,
	}
	if w := floatArg(m, "weight", 0); w > 0 {
		product["weight"] = w
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPost, "/products", url.Values{}, map[string]interface{}{"product": product})
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoUpdateProduct(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	sku := strArg(m, "sku")
	if sku == "" {
		return mcp.ToolResultError("sku is required")
	}

	product := map[string]interface{}{}
	if n := strArg(m, "name"); n != "" {
		product["name"] = n
	}
	if p := floatArg(m, "price", -1); p >= 0 {
		product["price"] = p
	}
	if s := intArg(m, "status", -1); s >= 0 {
		product["status"] = s
	}
	if v := intArg(m, "visibility", -1); v >= 0 {
		product["visibility"] = v
	}
	if w := floatArg(m, "weight", -1); w >= 0 {
		product["weight"] = w
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPut, "/products/"+url.PathEscape(sku), url.Values{}, map[string]interface{}{"product": product})
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoDeleteProduct(args json.RawMessage) map[string]interface{} {
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
	data, _, err := client.Request(c, http.MethodDelete, "/products/"+url.PathEscape(sku), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoListProductTypes(args json.RawMessage) map[string]interface{} {
	client, _, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/products/types", url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
