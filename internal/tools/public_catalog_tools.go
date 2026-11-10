package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerPublicCatalogTools(add toolAdder) {
	add(
		"magento_public_search_products",
		"Search and list products for guest shoppers without authentication via Magento GraphQL. Supports keyword search, category filter, price range, and returns product attributes with website stock status (IN_STOCK / OUT_OF_STOCK).",
		publicProps(map[string]interface{}{
			"search":       map[string]interface{}{"type": "string", "description": "Search keyword matching product name, SKU, or description"},
			"category_id":  map[string]interface{}{"type": "string", "description": "Filter by Magento category ID"},
			"price_from":   map[string]interface{}{"type": "number", "description": "Minimum price filter"},
			"price_to":     map[string]interface{}{"type": "number", "description": "Maximum price filter"},
			"page_size":    map[string]interface{}{"type": "integer", "description": "Number of products per page (default 20, max 50)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Current page number (default 1)"},
		}),
		nil,
		handleMagentoPublicSearchProducts,
	)

	add(
		"magento_public_get_product",
		"Retrieve full product details by SKU for guest shoppers without authentication via Magento GraphQL. Includes description, image gallery, available sizes, colors, and stock status.",
		publicProps(map[string]interface{}{
			"sku": map[string]interface{}{"type": "string", "description": "Product SKU"},
		}),
		[]string{"sku"},
		handleMagentoPublicGetProduct,
	)

	add(
		"magento_public_get_outfit",
		"Retrieve outfit styling recommendations (cross-sell items), similar alternatives (up-sell items), and accessories (related items) for a product SKU without authentication.",
		publicProps(map[string]interface{}{
			"sku": map[string]interface{}{"type": "string", "description": "Product SKU to find styling recommendations for"},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Recommendation type: 'crosssell' (outfit pairing), 'upsell' (similar items), 'related' (accessories), or 'all' (default)",
				"enum":        []interface{}{"crosssell", "upsell", "related", "all"},
			},
		}),
		[]string{"sku"},
		handleMagentoPublicGetOutfit,
	)

	add(
		"magento_public_list_categories",
		"Retrieve category hierarchy for guest shoppers to explore styles and catalog tree without authentication.",
		publicProps(map[string]interface{}{
			"category_id": map[string]interface{}{"type": "string", "description": "Parent category ID (optional, defaults to root catalog)"},
		}),
		nil,
		handleMagentoPublicListCategories,
	)

	add(
		"magento_public_guest_cart",
		"Guest cart management for shoppers without authentication. Create an empty cart, add items, or view current cart.",
		publicProps(map[string]interface{}{
			"action":  map[string]interface{}{"type": "string", "description": "Cart action: 'create', 'add', or 'get'", "enum": []interface{}{"create", "add", "get"}},
			"cart_id": map[string]interface{}{"type": "string", "description": "Guest Cart ID (required for 'add' and 'get')"},
			"sku":     map[string]interface{}{"type": "string", "description": "Product SKU to add (for 'add')"},
			"qty":     map[string]interface{}{"type": "number", "description": "Quantity to add (for 'add', default 1)"},
		}),
		[]string{"action"},
		handleMagentoPublicGuestCart,
	)
}

func handleMagentoPublicSearchProducts(args json.RawMessage) map[string]interface{} {
	client, m, err := graphqlClientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	search := strArg(m, "search")
	categoryID := strArg(m, "category_id")
	priceFrom := floatArg(m, "price_from", -1)
	priceTo := floatArg(m, "price_to", -1)
	pageSize := intArg(m, "page_size", 20)
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	currentPage := intArg(m, "current_page", 1)
	if currentPage <= 0 {
		currentPage = 1
	}

	variables := map[string]interface{}{
		"pageSize":    pageSize,
		"currentPage": currentPage,
	}
	if search != "" {
		variables["search"] = search
	}

	filter := map[string]interface{}{}
	if categoryID != "" {
		filter["category_id"] = map[string]interface{}{"eq": categoryID}
	}
	if priceFrom >= 0 || priceTo >= 0 {
		priceFilter := map[string]interface{}{}
		if priceFrom >= 0 {
			priceFilter["from"] = fmt.Sprintf("%.2f", priceFrom)
		}
		if priceTo >= 0 {
			priceFilter["to"] = fmt.Sprintf("%.2f", priceTo)
		}
		filter["price"] = priceFilter
	}
	if len(filter) > 0 {
		variables["filter"] = filter
	}

	query := `query SearchProducts($search: String, $filter: ProductAttributeFilterInput, $pageSize: Int, $currentPage: Int) {
		products(search: $search, filter: $filter, pageSize: $pageSize, currentPage: $currentPage) {
			total_count
			page_info {
				current_page
				total_pages
			}
			items {
				id
				sku
				name
				stock_status
				only_x_left_in_stock
				url_key
				small_image {
					url
					label
				}
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
	}`

	c, cancel := ctx()
	defer cancel()

	data, err := client.Query(c, query, variables)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoPublicGetProduct(args json.RawMessage) map[string]interface{} {
	client, m, err := graphqlClientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	sku := strArg(m, "sku")
	if sku == "" {
		return mcp.ToolResultError("sku is required")
	}

	query := `query GetProduct($sku: String!) {
		products(filter: { sku: { eq: $sku } }) {
			items {
				id
				sku
				name
				stock_status
				only_x_left_in_stock
				description {
					html
				}
				short_description {
					html
				}
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
				media_gallery {
					url
					label
					disabled
				}
				... on ConfigurableProduct {
					configurable_options {
						id
						attribute_id
						label
						attribute_code
						values {
							value_index
							label
							swatch_data {
								value
							}
						}
					}
					variants {
						attributes {
							label
							code
							value_index
						}
						product {
							id
							sku
							name
							stock_status
							price_range {
								minimum_price {
									final_price {
										value
										currency
									}
								}
							}
						}
					}
				}
			}
		}
	}`

	c, cancel := ctx()
	defer cancel()

	data, err := client.Query(c, query, map[string]interface{}{"sku": sku})
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoPublicGetOutfit(args json.RawMessage) map[string]interface{} {
	client, m, err := graphqlClientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	sku := strArg(m, "sku")
	if sku == "" {
		return mcp.ToolResultError("sku is required")
	}

	linkType := strings.ToLower(strArg(m, "type"))
	if linkType == "" {
		linkType = "all"
	}

	query := `query GetOutfit($sku: String!) {
		products(filter: { sku: { eq: $sku } }) {
			items {
				sku
				name
				crosssell_products {
					id
					sku
					name
					stock_status
					small_image {
						url
						label
					}
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
						}
					}
				}
				related_products {
					id
					sku
					name
					stock_status
					small_image {
						url
						label
					}
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
						}
					}
				}
				upsell_products {
					id
					sku
					name
					stock_status
					small_image {
						url
						label
					}
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
						}
					}
				}
			}
		}
	}`

	c, cancel := ctx()
	defer cancel()

	data, err := client.Query(c, query, map[string]interface{}{"sku": sku})
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	var resp struct {
		Products struct {
			Items []struct {
				SKU               string                   `json:"sku"`
				Name              string                   `json:"name"`
				CrosssellProducts []map[string]interface{} `json:"crosssell_products"`
				RelatedProducts   []map[string]interface{} `json:"related_products"`
				UpsellProducts    []map[string]interface{} `json:"upsell_products"`
			} `json:"items"`
		} `json:"products"`
	}

	if err := json.Unmarshal(data, &resp); err != nil {
		return rawResult(data)
	}

	if len(resp.Products.Items) == 0 {
		return jsonResult(map[string]interface{}{
			"sku":     sku,
			"message": "Product not found",
			"items":   []interface{}{},
		})
	}

	item := resp.Products.Items[0]
	result := map[string]interface{}{
		"source_sku":  item.SKU,
		"source_name": item.Name,
	}

	switch linkType {
	case "crosssell":
		result["crosssell_outfit"] = item.CrosssellProducts
		result["total"] = len(item.CrosssellProducts)
	case "upsell":
		result["upsell_alternatives"] = item.UpsellProducts
		result["total"] = len(item.UpsellProducts)
	case "related":
		result["related_accessories"] = item.RelatedProducts
		result["total"] = len(item.RelatedProducts)
	default:
		result["crosssell_outfit"] = item.CrosssellProducts
		result["upsell_alternatives"] = item.UpsellProducts
		result["related_accessories"] = item.RelatedProducts
		result["total"] = len(item.CrosssellProducts) + len(item.UpsellProducts) + len(item.RelatedProducts)
	}

	return jsonResult(result)
}

func handleMagentoPublicListCategories(args json.RawMessage) map[string]interface{} {
	client, m, err := graphqlClientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	catID := strArg(m, "category_id")
	variables := map[string]interface{}{}
	query := `query GetCategories {
		categoryList {
			id
			name
			url_path
			product_count
			children {
				id
				name
				url_path
				product_count
				children {
					id
					name
					url_path
					product_count
				}
			}
		}
	}`

	if catID != "" {
		variables["id"] = catID
		query = `query GetCategories($id: String) {
			categoryList(filters: { ids: { eq: $id } }) {
				id
				name
				url_path
				product_count
				children {
					id
					name
					url_path
					product_count
					children {
						id
						name
						url_path
						product_count
					}
				}
			}
		}`
	}

	c, cancel := ctx()
	defer cancel()

	data, err := client.Query(c, query, variables)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoPublicGuestCart(args json.RawMessage) map[string]interface{} {
	client, m, err := graphqlClientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	action := strings.ToLower(strArg(m, "action"))
	cartID := strArg(m, "cart_id")

	c, cancel := ctx()
	defer cancel()

	switch action {
	case "create":
		query := `mutation CreateCart { createEmptyCart }`
		data, err := client.Query(c, query, nil)
		if err != nil {
			return mcp.ToolResultError(err.Error())
		}
		var resp struct {
			CreateEmptyCart string `json:"createEmptyCart"`
		}
		if err := json.Unmarshal(data, &resp); err == nil && resp.CreateEmptyCart != "" {
			return jsonResult(map[string]interface{}{
				"cart_id": resp.CreateEmptyCart,
				"message": "Guest cart created successfully",
			})
		}
		return rawResult(data)

	case "add":
		if cartID == "" {
			return mcp.ToolResultError("cart_id is required for action 'add'")
		}
		sku := strArg(m, "sku")
		if sku == "" {
			return mcp.ToolResultError("sku is required for action 'add'")
		}
		qty := floatArg(m, "qty", 1)
		if qty <= 0 {
			qty = 1
		}

		mutation := `mutation AddToCart($cartId: String!, $cartItems: [CartItemInput!]!) {
			addProductsToCart(cartId: $cartId, cartItems: $cartItems) {
				cart {
					id
					total_quantity
					items {
						id
						quantity
						product {
							sku
							name
						}
						prices {
							price {
								value
								currency
							}
						}
					}
				}
				user_errors {
					message
				}
			}
		}`

		variables := map[string]interface{}{
			"cartId": cartID,
			"cartItems": []map[string]interface{}{
				{
					"sku":      sku,
					"quantity": qty,
				},
			},
		}

		data, err := client.Query(c, mutation, variables)
		if err != nil {
			return mcp.ToolResultError(err.Error())
		}
		return rawResult(data)

	case "get":
		if cartID == "" {
			return mcp.ToolResultError("cart_id is required for action 'get'")
		}
		query := `query GetCart($cartId: String!) {
			cart(cart_id: $cartId) {
				id
				is_virtual
				total_quantity
				items {
					id
					quantity
					product {
						sku
						name
						small_image {
							url
						}
					}
					prices {
						price {
							value
							currency
						}
					}
				}
				prices {
					grand_total {
						value
						currency
					}
				}
			}
		}`
		data, err := client.Query(c, query, map[string]interface{}{"cartId": cartID})
		if err != nil {
			return mcp.ToolResultError(err.Error())
		}
		return rawResult(data)

	default:
		return mcp.ToolResultError("invalid action; must be 'create', 'add', or 'get'")
	}
}
