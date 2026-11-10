package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/datumbridge/magento-mcp/internal/magento"
	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerProductTools(add toolAdder) {
	add(
		"magento_list_products",
		"Search and list products in Magento 2 using searchCriteria (pageSize, currentPage, sku, name, type_id, status, price_from, price_to, category_id).",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Number of products per page (default 20, max 100)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Current page number (default 1)"},
			"name":         map[string]interface{}{"type": "string", "description": "Filter products whose name contains this value"},
			"sku":          map[string]interface{}{"type": "string", "description": "Filter products whose SKU contains this value"},
			"category_id":  map[string]interface{}{"type": "string", "description": "Filter products belonging to this category ID"},
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
		"Retrieve full product details by SKU or product name. Provide sku if known; otherwise provide name to search and retrieve product details.",
		baseProps(map[string]interface{}{
			"sku":  map[string]interface{}{"type": "string", "description": "Product SKU (provide either sku or name)"},
			"name": map[string]interface{}{"type": "string", "description": "Product name or keyword to search if SKU is not known"},
		}),
		nil,
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
	if categoryID := strArg(m, "category_id"); categoryID != "" {
		fb.add("category_id", categoryID, "eq")
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

	mediaBase := getBaseMediaURL(client)
	var listResp map[string]interface{}
	if err := json.Unmarshal(data, &listResp); err == nil {
		if items, ok := listResp["items"].([]interface{}); ok {
			for _, it := range items {
				if prod, ok := it.(map[string]interface{}); ok {
					enrichProductMedia(prod, mediaBase)
				}
			}
			enrichProductsListPricing(c, client, items)
		}
		return jsonResult(listResp)
	}
	return rawResult(data)
}

func handleMagentoGetProduct(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	sku := strings.TrimSpace(strArg(m, "sku"))
	name := strings.TrimSpace(strArg(m, "name"))
	if sku == "" && name == "" {
		return mcp.ToolResultError("either sku or name is required")
	}

	c, cancel := ctx()
	defer cancel()

	if sku == "" {
		// Search product by name
		// Step 1: Query with visibility != 1 to exclude hidden child variants directly at API level
		q := url.Values{}
		fb := newFilterBuilder(q, 50, 1)
		fb.add("name", "%"+name+"%", "like")
		fb.add("visibility", "1", "neq")

		searchData, _, err := client.Request(c, http.MethodGet, "/products", q, nil)
		if err != nil {
			return mcp.ToolResultError("failed to search product by name: " + err.Error())
		}

		var searchResp struct {
			Items []map[string]interface{} `json:"items"`
			Total int                      `json:"total_count"`
		}
		if err := json.Unmarshal(searchData, &searchResp); err != nil {
			return mcp.ToolResultError("failed to parse product search response: " + err.Error())
		}

		// Fallback 1: If no products returned with visibility != 1, try searching without visibility filter
		if len(searchResp.Items) == 0 {
			qFallbackVis := url.Values{}
			fbFallbackVis := newFilterBuilder(qFallbackVis, 50, 1)
			fbFallbackVis.add("name", "%"+name+"%", "like")
			if fbData, _, err := client.Request(c, http.MethodGet, "/products", qFallbackVis, nil); err == nil {
				_ = json.Unmarshal(fbData, &searchResp)
			}
		}

		// Fallback 2: If still no items found and name contains multiple words, try searching for the longest word
		if len(searchResp.Items) == 0 {
			words := strings.Fields(name)
			var longest string
			for _, w := range words {
				wClean := strings.Trim(w, ",.?!:;\"'()[]{}")
				if len([]rune(wClean)) > len([]rune(longest)) {
					longest = wClean
				}
			}
			if len([]rune(longest)) >= 3 && !strings.EqualFold(longest, name) {
				qFallback := url.Values{}
				fbFallback := newFilterBuilder(qFallback, 50, 1)
				fbFallback.add("name", "%"+longest+"%", "like")
				fbFallback.add("visibility", "1", "neq")
				if fallbackData, _, err := client.Request(c, http.MethodGet, "/products", qFallback, nil); err == nil {
					_ = json.Unmarshal(fallbackData, &searchResp)
				}
				if len(searchResp.Items) == 0 {
					qKwAll := url.Values{}
					fbKwAll := newFilterBuilder(qKwAll, 50, 1)
					fbKwAll.add("name", "%"+longest+"%", "like")
					if fallbackData, _, err := client.Request(c, http.MethodGet, "/products", qKwAll, nil); err == nil {
						_ = json.Unmarshal(fallbackData, &searchResp)
					}
				}
			}
		}

		if len(searchResp.Items) == 0 {
			return mcp.ToolResultError(fmt.Sprintf("no product found matching name '%s'", name))
		}

		// Select the best candidate product, heavily prioritizing:
		// 1. Configurable parent products (they contain all variant options: colors, sizes, and stock)
		// 2. Visible products (visibility > 1: Catalog & Search) over hidden simple variants (visibility == 1)
		// 3. Exact name matches over suffixed variant names (e.g. "Sol" over "Sol - Đen - S")
		queryWords := strings.Fields(strings.ToLower(name))
		bestScore := -999999
		foundSKU := ""

		for _, item := range searchResp.Items {
			typeID, _ := item["type_id"].(string)
			vis := toInt(item["visibility"])
			itemName, _ := item["name"].(string)
			s, _ := item["sku"].(string)
			if s == "" {
				continue
			}

			score := 0

			// Type bonus: configurable products are parent products that carry all colors/sizes
			if typeID == "configurable" {
				score += 500
			}

			// Visibility bonus: catalog/search visible products are shopper-facing
			if vis == 4 { // Catalog & Search
				score += 400
			} else if vis == 2 || vis == 3 {
				score += 200
			} else if vis == 1 { // Not visible individually (hidden child variant)
				score -= 600
			}

			// Name matching score
			cleanItemName := strings.TrimSpace(itemName)
			if strings.EqualFold(cleanItemName, name) {
				score += 1000 // Exact name match
			} else {
				if strings.HasPrefix(strings.ToLower(cleanItemName), strings.ToLower(name)) {
					score += 200
				}
				for _, w := range queryWords {
					if strings.Contains(strings.ToLower(cleanItemName), w) {
						score += 20
					}
				}
				// Penalize extra variant suffix characters (e.g. " - Đen - S") if query didn't specify them
				if len([]rune(cleanItemName)) > len([]rune(name))+2 {
					score -= (len([]rune(cleanItemName)) - len([]rune(name))) * 2
				}
			}

			if score > bestScore {
				bestScore = score
				foundSKU = s
			}
		}

		if foundSKU == "" {
			return mcp.ToolResultError(fmt.Sprintf("unable to resolve product SKU for name '%s'", name))
		}
		sku = foundSKU
	}

	data, _, err := client.Request(c, http.MethodGet, "/products/"+url.PathEscape(sku), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	mediaBase := getBaseMediaURL(client)
	var prod map[string]interface{}
	if err := json.Unmarshal(data, &prod); err == nil {
		enrichProductMedia(prod, mediaBase)
		if typeID, _ := prod["type_id"].(string); typeID == "configurable" {
			enrichConfigurableProduct(c, client, prod, mediaBase)
		} else {
			enrichProductFromGraphQL(c, client, sku, prod)
		}
		return jsonResult(prod)
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

func enrichConfigurableProduct(c context.Context, client *magento.Client, prod map[string]interface{}, mediaBase string) {
	sku, _ := prod["sku"].(string)
	if sku == "" {
		return
	}
	if mediaBase == "" && client != nil {
		mediaBase = getBaseMediaURL(client)
	}

	// 0. Fetch GraphQL dynamic pricing (Catalog Price Rules) and swatches
	swatchesByLabel, swatchesByValIdx := enrichProductFromGraphQL(c, client, sku, prod)

	// 1. Fetch children to resolve price, special_price, and variants
	var children []map[string]interface{}
	if client != nil {
		childData, _, err := client.Request(c, http.MethodGet, "/configurable-products/"+url.PathEscape(sku)+"/children", url.Values{}, nil)
		if err == nil {
			if err := json.Unmarshal(childData, &children); err == nil && len(children) > 0 {
				var minPrice float64 = -1
				var maxPrice float64 = -1
				var specialPrice float64 = -1
				var regularPrice float64 = -1

				for _, child := range children {
					cp := toFloat(child["price"])
					if cp > 0 {
						if minPrice < 0 || cp < minPrice {
							minPrice = cp
						}
						if cp > maxPrice {
							maxPrice = cp
						}
						if regularPrice < 0 {
							regularPrice = cp
						}
					}
					if cas, ok := child["custom_attributes"].([]interface{}); ok {
						for _, item := range cas {
							if cm, ok := item.(map[string]interface{}); ok {
								if cm["attribute_code"] == "special_price" {
									sp := toFloat(cm["value"])
									if sp > 0 && (specialPrice < 0 || sp < specialPrice) {
										specialPrice = sp
									}
								}
							}
						}
					}
				}

				// Only calculate from children if GraphQL didn't already resolve catalog rule pricing
				if prod["original_price"] == nil {
					parentPrice := toFloat(prod["price"])
					if parentPrice == 0 {
						if specialPrice > 0 {
							prod["price"] = specialPrice
							prod["original_price"] = regularPrice
							prod["special_price"] = specialPrice
						} else if minPrice > 0 {
							prod["price"] = minPrice
							prod["original_price"] = regularPrice
						}
					} else if specialPrice > 0 {
						prod["special_price"] = specialPrice
						if parentPrice > specialPrice {
							prod["original_price"] = parentPrice
						}
					}
				}
				if minPrice > 0 && prod["price_min"] == nil {
					prod["price_min"] = minPrice
					prod["price_max"] = maxPrice
				}
			}
		}
	}

	// 2. Resolve option labels for configurable_product_options (Color, Size)
	ext, ok := prod["extension_attributes"].(map[string]interface{})
	if !ok {
		return
	}
	opts, ok := ext["configurable_product_options"].([]interface{})
	if !ok || len(opts) == 0 {
		return
	}

	var colors []string
	var sizes []string

	type colorOptionItem struct {
		Index    string
		RawIndex interface{}
		Label    string
		VMap     map[string]interface{}
	}
	var colorItems []colorOptionItem

	for _, optItem := range opts {
		optMap, ok := optItem.(map[string]interface{})
		if !ok {
			continue
		}
		attrID := fmt.Sprintf("%v", optMap["attribute_id"])
		label, _ := optMap["label"].(string)

		labelMap := make(map[string]string)
		if client != nil {
			optData, _, err := client.Request(c, http.MethodGet, "/products/attributes/"+url.PathEscape(attrID)+"/options", url.Values{}, nil)
			if err == nil {
				var attrOptions []map[string]interface{}
				if err := json.Unmarshal(optData, &attrOptions); err == nil {
					for _, ao := range attrOptions {
						valStr := fmt.Sprintf("%v", ao["value"])
						lblStr, _ := ao["label"].(string)
						if valStr != "" && lblStr != "" {
							labelMap[valStr] = lblStr
						}
					}
				}
			}
		}

		lowerLbl := strings.ToLower(label)
		isColor := strings.Contains(lowerLbl, "color") || strings.Contains(lowerLbl, "màu") || strings.Contains(lowerLbl, "mau")
		isSize := strings.Contains(lowerLbl, "size") || strings.Contains(lowerLbl, "kích") || strings.Contains(lowerLbl, "kich")

		if vals, ok := optMap["values"].([]interface{}); ok {
			for _, vItem := range vals {
				vMap, ok := vItem.(map[string]interface{})
				if !ok {
					continue
				}
				rawIdx := vMap["value_index"]
				vIdx := fmt.Sprintf("%v", rawIdx)
				if lbl, found := labelMap[vIdx]; found {
					vMap["label"] = lbl
				}
				lbl, _ := vMap["label"].(string)
				if lbl == "" {
					lbl = vIdx
				}

				if isColor {
					colors = append(colors, lbl)
					colorItems = append(colorItems, colorOptionItem{
						Index:    vIdx,
						RawIndex: rawIdx,
						Label:    lbl,
						VMap:     vMap,
					})
				} else if isSize {
					sizes = append(sizes, lbl)
				}
			}
		}
	}

	if len(colors) > 0 {
		prod["available_colors"] = colors
	}
	if len(sizes) > 0 {
		prod["available_sizes"] = sizes
	}

	// 3. Group images by color variant
	if len(colorItems) > 0 {
		type colorGalleryCollector struct {
			label     string
			vIdx      string
			rawIdx    interface{}
			vMap      map[string]interface{}
			mainImage string
			gallery   []string
			seen      map[string]bool
		}

		collectorsByVIdx := make(map[string]*colorGalleryCollector)
		var collectorList []*colorGalleryCollector

		for _, item := range colorItems {
			col := &colorGalleryCollector{
				label:   item.Label,
				vIdx:    item.Index,
				rawIdx:  item.RawIndex,
				vMap:    item.VMap,
				gallery: make([]string, 0),
				seen:    make(map[string]bool),
			}
			collectorsByVIdx[item.Index] = col
			collectorList = append(collectorList, col)
		}

		addImage := func(col *colorGalleryCollector, imgURL string) {
			clean := strings.TrimSpace(imgURL)
			if clean == "" || col.seen[clean] {
				return
			}
			col.seen[clean] = true
			col.gallery = append(col.gallery, clean)
			if col.mainImage == "" {
				col.mainImage = clean
			}
		}

		// A. Collect from children (simple products)
		for _, child := range children {
			enrichProductMedia(child, mediaBase)

			var matchedVIdx string
			if cas, ok := child["custom_attributes"].([]interface{}); ok {
				for _, it := range cas {
					if cm, ok := it.(map[string]interface{}); ok {
						valStr := fmt.Sprintf("%v", cm["value"])
						if _, exists := collectorsByVIdx[valStr]; exists {
							matchedVIdx = valStr
							break
						}
					}
				}
			}

			if matchedVIdx != "" {
				col := collectorsByVIdx[matchedVIdx]
				if childImg, _ := child["image_url"].(string); childImg != "" {
					addImage(col, childImg)
				}
				if rawEntries, ok := child["media_gallery_entries"].([]interface{}); ok {
					for _, entryItem := range rawEntries {
						if entryMap, ok := entryItem.(map[string]interface{}); ok {
							if u, _ := entryMap["url"].(string); u != "" {
								addImage(col, u)
							} else if f, _ := entryMap["file"].(string); f != "" {
								addImage(col, f)
							}
						}
					}
				}
			}
		}

		// B. Collect from parent's media_gallery_entries with matching labels
		if parentEntries, ok := prod["media_gallery_entries"].([]interface{}); ok {
			for _, entryItem := range parentEntries {
				entryMap, ok := entryItem.(map[string]interface{})
				if !ok {
					continue
				}
				imgURL, _ := entryMap["url"].(string)
				if imgURL == "" {
					imgURL, _ = entryMap["file"].(string)
				}
				if imgURL == "" {
					continue
				}
				entryLabel := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", entryMap["label"])))
				if entryLabel != "" {
					for _, col := range collectorList {
						if strings.Contains(entryLabel, strings.ToLower(col.label)) {
							addImage(col, imgURL)
						}
					}
				}
			}
		}

		// C. Fallback: If no color has images yet, check if parent represents a specific color
		hasAnyImages := false
		for _, col := range collectorList {
			if len(col.gallery) > 0 {
				hasAnyImages = true
				break
			}
		}

		if !hasAnyImages {
			parentContext := strings.ToLower(fmt.Sprintf("%v %v", prod["url_key"], prod["name"]))
			if cas, ok := prod["custom_attributes"].([]interface{}); ok {
				for _, it := range cas {
					if cm, ok := it.(map[string]interface{}); ok {
						code, _ := cm["attribute_code"].(string)
						if code == "url_key" || code == "short_description" || code == "description" || code == "meta_description" {
							parentContext += " " + strings.ToLower(fmt.Sprintf("%v", cm["value"]))
						}
					}
				}
			}

			var matchedCol *colorGalleryCollector
			matchedCount := 0
			for _, col := range collectorList {
				if strings.Contains(parentContext, strings.ToLower(col.label)) {
					matchedCol = col
					matchedCount++
				}
			}

			if (matchedCount == 1 && matchedCol != nil) || len(collectorList) == 1 {
				targetCol := matchedCol
				if targetCol == nil && len(collectorList) == 1 {
					targetCol = collectorList[0]
				}

				if pImg, _ := prod["image_url"].(string); pImg != "" {
					addImage(targetCol, pImg)
				}
				if parentEntries, ok := prod["media_gallery_entries"].([]interface{}); ok {
					for _, entryItem := range parentEntries {
						if entryMap, ok := entryItem.(map[string]interface{}); ok {
							if u, _ := entryMap["url"].(string); u != "" {
								addImage(targetCol, u)
							} else if f, _ := entryMap["file"].(string); f != "" {
								addImage(targetCol, f)
							}
						}
					}
				}
			}
		}

		// D. Output color_images, root colors, and enrich option value maps
		var colorImages []map[string]interface{}
		var rootColors []map[string]interface{}
		for _, col := range collectorList {
			hexCode := swatchesByLabel[strings.ToLower(strings.TrimSpace(col.label))]
			if hexCode == "" {
				hexCode = swatchesByValIdx[col.vIdx]
			}
			if hexCode == "" {
				hexCode = resolveColorHex(col.label)
			}

			item := map[string]interface{}{
				"color":       col.label,
				"color_code":  hexCode,
				"value_index": col.rawIdx,
				"image_url":   col.mainImage,
				"gallery":     col.gallery,
			}
			colorImages = append(colorImages, item)

			rootColors = append(rootColors, map[string]interface{}{
				"name":      col.label,
				"code":      hexCode,
				"imageUrl":  col.mainImage,
				"image_url": col.mainImage,
			})

			if col.vMap != nil {
				col.vMap["color_code"] = hexCode
				col.vMap["image_url"] = col.mainImage
				col.vMap["gallery"] = col.gallery
			}
		}

		if len(colorImages) > 0 {
			prod["color_images"] = colorImages
		}
		if len(rootColors) > 0 {
			prod["colors"] = rootColors
		}
	}
}

func enrichProductFromGraphQL(ctx context.Context, client *magento.Client, sku string, prod map[string]interface{}) (map[string]string, map[string]string) {
	byLabel := make(map[string]string)
	byValIdx := make(map[string]string)
	if client == nil || strings.TrimSpace(sku) == "" || prod == nil {
		return byLabel, byValIdx
	}

	gqlQuery := fmt.Sprintf(`query {
  products(filter: { sku: { eq: %q } }) {
    items {
      price_range {
        minimum_price {
          regular_price {
            value
            currency
          }
          final_price {
            value
            currency
          }
          discount {
            amount_off
            percent_off
          }
        }
        maximum_price {
          regular_price {
            value
            currency
          }
          final_price {
            value
            currency
          }
          discount {
            amount_off
            percent_off
          }
        }
      }
      ... on ConfigurableProduct {
        configurable_options {
          attribute_code
          values {
            value_index
            label
            swatch_data {
              value
            }
          }
        }
      }
    }
  }
}`, sku)

	data, err := client.GraphQL(ctx, gqlQuery, nil)
	if err != nil || len(data) == 0 {
		return byLabel, byValIdx
	}

	var resp struct {
		Data struct {
			Products struct {
				Items []struct {
					PriceRange struct {
						MinimumPrice struct {
							RegularPrice struct {
								Value    float64 `json:"value"`
								Currency string  `json:"currency"`
							} `json:"regular_price"`
							FinalPrice struct {
								Value    float64 `json:"value"`
								Currency string  `json:"currency"`
							} `json:"final_price"`
							Discount struct {
								AmountOff  float64 `json:"amount_off"`
								PercentOff float64 `json:"percent_off"`
							} `json:"discount"`
						} `json:"minimum_price"`
						MaximumPrice struct {
							RegularPrice struct {
								Value    float64 `json:"value"`
								Currency string  `json:"currency"`
							} `json:"regular_price"`
							FinalPrice struct {
								Value    float64 `json:"value"`
								Currency string  `json:"currency"`
							} `json:"final_price"`
						} `json:"maximum_price"`
					} `json:"price_range"`
					ConfigurableOptions []struct {
						AttributeCode string `json:"attribute_code"`
						Values        []struct {
							ValueIndex int    `json:"value_index"`
							Label      string `json:"label"`
							SwatchData *struct {
								Value string `json:"value"`
							} `json:"swatch_data"`
						} `json:"values"`
					} `json:"configurable_options"`
				} `json:"items"`
			} `json:"products"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return byLabel, byValIdx
	}

	for _, item := range resp.Data.Products.Items {
		minP := item.PriceRange.MinimumPrice
		regPrice := minP.RegularPrice.Value
		finPrice := minP.FinalPrice.Value
		pctOff := minP.Discount.PercentOff
		amtOff := minP.Discount.AmountOff

		if finPrice > 0 && regPrice > 0 && finPrice < regPrice {
			prod["price"] = finPrice
			prod["final_price"] = finPrice
			prod["special_price"] = finPrice
			prod["original_price"] = regPrice
			prod["regular_price"] = regPrice
			if pctOff > 0 {
				prod["discount"] = fmt.Sprintf("-%d%%", int(pctOff))
				prod["discount_percent"] = pctOff
			}
			if amtOff > 0 {
				prod["discount_amount"] = amtOff
			}
		} else if regPrice > 0 {
			if toFloat(prod["price"]) == 0 {
				prod["price"] = regPrice
			}
			prod["regular_price"] = regPrice
		}

		if minP.RegularPrice.Currency != "" && prod["currency"] == nil {
			curr := minP.RegularPrice.Currency
			if curr == "VND" {
				prod["currency"] = "đ"
			} else {
				prod["currency"] = curr
			}
		}

		for _, opt := range item.ConfigurableOptions {
			for _, val := range opt.Values {
				if val.SwatchData != nil && val.SwatchData.Value != "" {
					swatchVal := strings.TrimSpace(val.SwatchData.Value)
					if strings.HasPrefix(swatchVal, "#") || (len(swatchVal) == 6 && isHex(swatchVal)) {
						if !strings.HasPrefix(swatchVal, "#") {
							swatchVal = "#" + swatchVal
						}
						lblLower := strings.ToLower(strings.TrimSpace(val.Label))
						if lblLower != "" {
							byLabel[lblLower] = swatchVal
						}
						vIdxStr := strconv.Itoa(val.ValueIndex)
						byValIdx[vIdxStr] = swatchVal
					}
				}
			}
		}
	}

	return byLabel, byValIdx
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func resolveColorHex(colorName string) string {
	lower := strings.ToLower(strings.TrimSpace(colorName))
	switch {
	case strings.Contains(lower, "latte"):
		return "#b58f72"
	case strings.Contains(lower, "denim"):
		return "#6d8295"
	case strings.Contains(lower, "olive"):
		return "#808000"
	case strings.Contains(lower, "rượu") || strings.Contains(lower, "vang") || strings.Contains(lower, "wine") || strings.Contains(lower, "burgundy") || strings.Contains(lower, "bordeaux"):
		return "#722f37"
	case strings.Contains(lower, "mận") || strings.Contains(lower, "plum") || strings.Contains(lower, "đỏ đô") || strings.Contains(lower, "maroon"):
		return "#701c24"
	case strings.Contains(lower, "nâu") || strings.Contains(lower, "mocha") || strings.Contains(lower, "brown") || strings.Contains(lower, "chocolate") || strings.Contains(lower, "cafe"):
		return "#806454"
	case strings.Contains(lower, "sage"):
		return "#9caf88"
	case strings.Contains(lower, "pastel") || strings.Contains(lower, "baby blue") || strings.Contains(lower, "nhạt"):
		return "#b7dccb"
	case strings.Contains(lower, "rêu") || strings.Contains(lower, "moss"):
		return "#5e7153"
	case strings.Contains(lower, "navy") || strings.Contains(lower, "xanh đen") || strings.Contains(lower, "than"):
		return "#1e3a5f"
	case strings.Contains(lower, "trắng") || strings.Contains(lower, "white") || strings.Contains(lower, "kem") || strings.Contains(lower, "ngà"):
		return "#ffffff"
	case strings.Contains(lower, "xám") || strings.Contains(lower, "grey") || strings.Contains(lower, "gray") || strings.Contains(lower, "ghi"):
		return "#94a3b8"
	case strings.Contains(lower, "be") || strings.Contains(lower, "beige") || strings.Contains(lower, "cream"):
		return "#e4dac2"
	case strings.Contains(lower, "hồng") || strings.Contains(lower, "pink"):
		return "#f472b6"
	case strings.Contains(lower, "đỏ") || strings.Contains(lower, "red"):
		return "#dc2626"
	case strings.Contains(lower, "vàng") || strings.Contains(lower, "yellow"):
		return "#eab308"
	case strings.Contains(lower, "cam") || strings.Contains(lower, "orange"):
		return "#f97316"
	case strings.Contains(lower, "tím") || strings.Contains(lower, "purple"):
		return "#a855f7"
	case strings.Contains(lower, "đen") || strings.Contains(lower, "black"):
		return "#18181b"
	case strings.Contains(lower, "xanh") || strings.Contains(lower, "blue"):
		return "#2563eb"
	default:
		return "#64748b"
	}
}

func toFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	default:
		return 0
	}
}

func toInt(v interface{}) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		var n int
		fmt.Sscanf(val, "%d", &n)
		return n
	default:
		return 0
	}
}

func enrichProductsListPricing(ctx context.Context, client *magento.Client, items []interface{}) {
	if len(items) == 0 {
		return
	}

	var skus []string
	prodBySKU := make(map[string]map[string]interface{})

	for _, it := range items {
		prod, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		sku, _ := prod["sku"].(string)
		sku = strings.TrimSpace(sku)
		if sku != "" {
			skus = append(skus, sku)
			prodBySKU[sku] = prod
		}

		// 1. Fallback from custom_attributes (special_price & discount_percent)
		var sp float64 = -1
		var discPct float64 = -1
		if cas, ok := prod["custom_attributes"].([]interface{}); ok {
			for _, item := range cas {
				if cm, ok := item.(map[string]interface{}); ok {
					code, _ := cm["attribute_code"].(string)
					if code == "special_price" {
						sp = toFloat(cm["value"])
					} else if code == "discount_percent" {
						discPct = toFloat(cm["value"])
					}
				}
			}
		}

		basePrice := toFloat(prod["price"])
		if sp > 0 && (basePrice <= 0 || sp < basePrice) {
			prod["price"] = sp
			prod["special_price"] = sp
			prod["final_price"] = sp
			if basePrice > sp {
				prod["original_price"] = basePrice
				prod["regular_price"] = basePrice
				pct := int(math.Round((basePrice - sp) / basePrice * 100))
				prod["discount"] = fmt.Sprintf("-%d%%", pct)
				prod["discount_percent"] = pct
			}
		} else if discPct > 0 && basePrice > 0 {
			salePrice := math.Round(basePrice * (1 - discPct/100))
			prod["price"] = salePrice
			prod["special_price"] = salePrice
			prod["final_price"] = salePrice
			prod["original_price"] = basePrice
			prod["regular_price"] = basePrice
			prod["discount"] = fmt.Sprintf("-%d%%", int(discPct))
			prod["discount_percent"] = discPct
		}
	}

	// 2. Batch GraphQL query for catalog price rules & configurable product prices
	if client == nil || len(skus) == 0 {
		return
	}

	var formattedSKUs []string
	for _, s := range skus {
		formattedSKUs = append(formattedSKUs, fmt.Sprintf("%q", s))
	}

	gqlQuery := fmt.Sprintf(`query {
  products(filter: { sku: { in: [%s] } }, pageSize: %d) {
    items {
      sku
      price_range {
        minimum_price {
          regular_price {
            value
            currency
          }
          final_price {
            value
            currency
          }
          discount {
            amount_off
            percent_off
          }
        }
      }
    }
  }
}`, strings.Join(formattedSKUs, ", "), len(skus)+5)

	data, err := client.GraphQL(ctx, gqlQuery, nil)
	if err != nil || len(data) == 0 {
		return
	}

	var resp struct {
		Data struct {
			Products struct {
				Items []struct {
					SKU        string `json:"sku"`
					PriceRange struct {
						MinimumPrice struct {
							RegularPrice struct {
								Value    float64 `json:"value"`
								Currency string  `json:"currency"`
							} `json:"regular_price"`
							FinalPrice struct {
								Value    float64 `json:"value"`
								Currency string  `json:"currency"`
							} `json:"final_price"`
							Discount struct {
								AmountOff  float64 `json:"amount_off"`
								PercentOff float64 `json:"percent_off"`
							} `json:"discount"`
						} `json:"minimum_price"`
					} `json:"price_range"`
				} `json:"items"`
			} `json:"products"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}

	for _, gItem := range resp.Data.Products.Items {
		prod, exists := prodBySKU[gItem.SKU]
		if !exists || prod == nil {
			continue
		}

		minP := gItem.PriceRange.MinimumPrice
		regPrice := minP.RegularPrice.Value
		finPrice := minP.FinalPrice.Value
		pctOff := minP.Discount.PercentOff
		amtOff := minP.Discount.AmountOff

		if finPrice > 0 && regPrice > 0 && finPrice < regPrice {
			prod["price"] = finPrice
			prod["final_price"] = finPrice
			prod["special_price"] = finPrice
			prod["original_price"] = regPrice
			prod["regular_price"] = regPrice
			if pctOff > 0 {
				prod["discount"] = fmt.Sprintf("-%d%%", int(pctOff))
				prod["discount_percent"] = pctOff
			} else {
				calcPct := int(math.Round((regPrice - finPrice) / regPrice * 100))
				if calcPct > 0 {
					prod["discount"] = fmt.Sprintf("-%d%%", calcPct)
					prod["discount_percent"] = calcPct
				}
			}
			if amtOff > 0 {
				prod["discount_amount"] = amtOff
			}
		} else if regPrice > 0 {
			if toFloat(prod["price"]) == 0 {
				prod["price"] = regPrice
			}
			prod["regular_price"] = regPrice
		}

		if minP.RegularPrice.Currency != "" && prod["currency"] == nil {
			curr := minP.RegularPrice.Currency
			if curr == "VND" {
				prod["currency"] = "đ"
			} else {
				prod["currency"] = curr
			}
		}
	}
}

