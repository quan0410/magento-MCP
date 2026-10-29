package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerProductTools(add toolAdder) {
	add(
		"magento_list_products",
		"Search and list products in Magento 2 using searchCriteria (pageSize, currentPage, sku, name, type_id, status, price_from, price_to).",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Number of products per page (default 20, max 100)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Current page number (default 1)"},
			"name":         map[string]interface{}{"type": "string", "description": "Filter products whose name contains this value"},
			"sku":          map[string]interface{}{"type": "string", "description": "Filter products whose SKU contains this value"},
			"type_id":      map[string]interface{}{"type": "string", "description": "Filter by product type (e.g. simple, configurable, virtual)"},
			"status":       map[string]interface{}{"type": "integer", "description": "Filter by status: 1 for Enabled, 2 for Disabled"},
			"price_from":   map[string]interface{}{"type": "number", "description": "Minimum product price filter"},
			"price_to":     map[string]interface{}{"type": "number", "description": "Maximum product price filter"},
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

	add(
		"magento_get_related_products",
		"Retrieve related, cross-sell, or up-sell products linked to a product SKU for outfit styling or recommendation.",
		baseProps(map[string]interface{}{
			"sku": map[string]interface{}{"type": "string", "description": "Product SKU to find linked products for"},
			"link_type": map[string]interface{}{
				"type":        "string",
				"description": "Type of product link: 'crosssell' (outfit pairing / bundle), 'related' (similar items), or 'upsell' (premium upgrade). Default is 'crosssell'",
				"enum":        []interface{}{"crosssell", "related", "upsell"},
			},
			"limit": map[string]interface{}{"type": "integer", "description": "Maximum number of linked products to return (default 4, max 20)"},
		}),
		[]string{"sku"},
		handleMagentoGetRelatedProducts,
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
	fb := newFilterBuilder(q, pageSize, currentPage)

	if name := strArg(m, "name"); name != "" {
		fb.add("name", "%"+name+"%", "like")
	}
	if sku := strArg(m, "sku"); sku != "" {
		fb.add("sku", "%"+sku+"%", "like")
	}
	if typeID := strArg(m, "type_id"); typeID != "" {
		fb.add("type_id", typeID, "eq")
	}
	if status := intArg(m, "status", 0); status > 0 {
		fb.add("status", strconv.Itoa(status), "eq")
	}
	if priceFrom := floatArg(m, "price_from", -1); priceFrom >= 0 {
		fb.add("price", fmt.Sprintf("%.2f", priceFrom), "gteq")
	}
	if priceTo := floatArg(m, "price_to", -1); priceTo >= 0 {
		fb.add("price", fmt.Sprintf("%.2f", priceTo), "lteq")
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

func handleMagentoGetRelatedProducts(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	sku := strArg(m, "sku")
	if sku == "" {
		return mcp.ToolResultError("sku is required")
	}

	linkType := strings.ToLower(strArg(m, "link_type"))
	if linkType == "" {
		linkType = "crosssell"
	}
	if linkType != "crosssell" && linkType != "related" && linkType != "upsell" {
		return mcp.ToolResultError("link_type must be one of: 'crosssell', 'related', 'upsell'")
	}
	limit := intArg(m, "limit", 4)
	if limit <= 0 {
		limit = 4
	}
	if limit > 20 {
		limit = 20
	}

	c, cancel := ctx()
	defer cancel()

	endpoint := fmt.Sprintf("/products/%s/links/%s", url.PathEscape(sku), url.PathEscape(linkType))
	data, _, err := client.Request(c, http.MethodGet, endpoint, url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(fmt.Sprintf("Failed to fetch %s links for SKU '%s': %v", linkType, sku, err))
	}

	var rawLinks []map[string]interface{}
	if err := json.Unmarshal(data, &rawLinks); err != nil {
		return mcp.ToolResultError("Failed to parse Magento product links: " + err.Error())
	}

	if len(rawLinks) == 0 {
		return jsonResult(map[string]interface{}{
			"source_sku":  sku,
			"link_type":   linkType,
			"items":       []interface{}{},
			"total_count": 0,
			"message":     fmt.Sprintf("Sản phẩm '%s' hiện chưa có liên kết %s nào được thiết lập.", sku, linkType),
		})
	}

	// Collect linked SKUs up to limit
	var linkedSKUs []string
	for _, l := range rawLinks {
		if linkedSKU, ok := l["linked_product_sku"].(string); ok && linkedSKU != "" {
			linkedSKUs = append(linkedSKUs, linkedSKU)
			if len(linkedSKUs) >= limit {
				break
			}
		}
	}

	if len(linkedSKUs) == 0 {
		return jsonResult(map[string]interface{}{
			"source_sku":  sku,
			"link_type":   linkType,
			"items":       []interface{}{},
			"total_count": 0,
		})
	}

	// Fetch full details of linked SKUs
	q := url.Values{}
	fb := newFilterBuilder(q, len(linkedSKUs), 1)
	fb.addIn("sku", linkedSKUs)

	prodData, _, err := client.Request(c, http.MethodGet, "/products", q, nil)
	if err != nil {
		var fallbackItems []map[string]interface{}
		for _, s := range linkedSKUs {
			fallbackItems = append(fallbackItems, map[string]interface{}{
				"sku": s,
			})
		}
		return jsonResult(map[string]interface{}{
			"source_sku":  sku,
			"link_type":   linkType,
			"items":       fallbackItems,
			"total_count": len(fallbackItems),
		})
	}

	var prodResp struct {
		Items []map[string]interface{}
	}
	if err := json.Unmarshal(prodData, &prodResp); err != nil {
		return mcp.ToolResultError("Failed to parse linked products details: " + err.Error())
	}

	getImage := func(prod map[string]interface{}) string {
		if attrs, ok := prod["custom_attributes"].([]interface{}); ok {
			for _, a := range attrs {
				if attr, ok := a.(map[string]interface{}); ok {
					code, _ := attr["attribute_code"].(string)
					if code == "thumbnail" || code == "image" || code == "small_image" {
						val, _ := attr["value"].(string)
						if val != "" && val != "no_selection" {
							return val
						}
					}
				}
			}
		}
		return ""
	}

	var items []map[string]interface{}
	for _, p := range prodResp.Items {
		items = append(items, map[string]interface{}{
			"sku":       p["sku"],
			"name":      p["name"],
			"price":     p["price"],
			"status":    p["status"],
			"type_id":   p["type_id"],
			"image_url": getImage(p),
		})
	}

	return jsonResult(map[string]interface{}{
		"source_sku":  sku,
		"link_type":   linkType,
		"items":       items,
		"total_count": len(items),
	})
}
