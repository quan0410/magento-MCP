package tools

import (
	"strings"

	"github.com/datumbridge/magento-mcp/internal/mcp"
	registrydocs "github.com/datumbridge/magento-mcp/registry_docs"
)

var toolOutputs = map[string]string{
	// Products
	"magento_list_products":               "items, total_count",
	"magento_get_product":                 "id, sku, name, price, status, type_id",
	"magento_create_product":              "id, sku, name, price, status",
	"magento_update_product":              "id, sku, name, price, status",
	"magento_delete_product":              "",
	"magento_list_product_types":          "name, label",
	"magento_get_product_import_template": "format, filename, file_base64, fields_guide",
	"magento_import_create_products":      "total_rows, created_count, failed_count, failed_items, warnings",
	"magento_import_update_products":      "total_rows, updated_count, failed_count, failed_items, warnings",
	"magento_get_media_import_status":     "is_running, total_queued, completed, failed, pending, recent_warnings",
	"magento_get_related_products":        "source_sku, link_type, items",
	// Orders
	"magento_list_orders":       "items, total_count",
	"magento_get_order":         "entity_id, increment_id, status, grand_total, items",
	"magento_add_order_comment": "",
	"magento_cancel_order":      "",
	"magento_hold_order":        "",
	"magento_verify_and_get_order": "verified, order_number, status, total_amount, shipping, items",
	// Customers
	"magento_list_customers":  "items, total_count",
	"magento_get_customer":    "id, email, firstname, lastname, group_id",
	"magento_create_customer": "id, email, firstname, lastname, group_id",
	"magento_update_customer": "id, email, firstname, lastname, group_id",
	// Inventory
	"magento_get_stock_item":         "item_id, product_id, qty, is_in_stock",
	"magento_update_stock_item":      "item_id, product_id, qty, is_in_stock",
	"magento_list_inventory_sources": "items, total_count",
	"magento_get_source_items":       "items, total_count",
	// Categories
	"magento_list_categories":         "id, parent_id, name, children_data",
	"magento_get_category":            "id, parent_id, name, position",
	"magento_list_category_products":  "sku, position, category_id",
	"magento_assign_category_product": "",
	// Invoices & Shipments
	"magento_list_invoices":   "items, total_count",
	"magento_get_invoice":     "entity_id, order_id, grand_total, state",
	"magento_create_shipment": "shipment_id, order_id",
	// Store
	"magento_list_store_views":   "id, code, name, website_id",
	"magento_list_store_configs": "items",
	// Promotions
	"magento_list_sales_rules": "items, total_count",
	"magento_get_sales_rule":   "rule_id, name, coupon_type, discount_amount",
	"magento_delete_coupon":    "",
	// Credit Memos
	"magento_list_credit_memos":  "items, total_count",
	"magento_get_credit_memo":    "entity_id, order_id, grand_total, items",
	"magento_create_credit_memo": "credit_memo_id, order_id",
	// CMS
	"magento_list_cms_pages":  "items, total_count",
	"magento_get_cms_page":    "id, identifier, title, content",
	"magento_list_cms_blocks": "items, total_count",
	"magento_get_cms_block":   "id, identifier, title, content",
	// Product Media
	"magento_list_product_media": "id, media_type, file",
	"magento_get_product_media":  "id, media_type, file",
	// Cart
	"magento_create_empty_cart": "cart_id",
	"magento_get_cart":          "id, is_active, items, items_count",
	"magento_add_cart_item":     "item_id, sku, qty, quote_id",
}

type relatedMeta struct {
	tool  string
	field string
}

var paramRelated = map[string]relatedMeta{
	"sku":           {tool: "magento_list_products", field: "items[0].sku"},
	"order_id":      {tool: "magento_list_orders", field: "items[0].entity_id"},
	"customer_id":   {tool: "magento_list_customers", field: "items[0].id"},
	"category_id":   {tool: "magento_list_categories", field: "id"},
	"invoice_id":    {tool: "magento_list_invoices", field: "items[0].entity_id"},
	"creditmemo_id": {tool: "magento_list_credit_memos", field: "items[0].entity_id"},
	"rule_id":       {tool: "magento_list_sales_rules", field: "items[0].rule_id"},
	"page_id":       {tool: "magento_list_cms_pages", field: "items[0].id"},
	"block_id":      {tool: "magento_list_cms_blocks", field: "items[0].id"},
	"cart_id":       {tool: "magento_create_empty_cart", field: "cart_id"},
	"entry_id":      {tool: "magento_list_product_media", field: "[0].id"},
	"source_code":   {tool: "magento_list_inventory_sources", field: "items[0].source_code"},
}

func annotateRelated(toolName string, props map[string]interface{}) {
	for k, v := range props {
		propMap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if k == "sku" && toolName == "magento_create_product" {
			continue
		}
		if rel, found := paramRelated[k]; found {
			propMap["x-datumbridge-related-tool"] = rel.tool
			propMap["x-datumbridge-output-field"] = rel.field
		}
	}
}

func formatToolDescription(name, desc string) string {
	res := strings.TrimSpace(desc)
	if !strings.Contains(strings.ToLower(res), "capabilities:") {
		res += "\n\nCapabilities: magento." + name
	}
	if !strings.Contains(strings.ToLower(res), "outputs:") {
		outs := toolOutputs[name]
		if outs != "" {
			res += "\nOutputs: " + outs + ", success"
		} else {
			res += "\nOutputs: success"
		}
	}
	return res
}

// Register returns the full magento-mcp tool catalogue: products, orders,
// customers, inventory, categories, invoices/shipments, and store configs.
// Full scope is the default; MAGENTO_TOOLS narrows it to an explicit allowlist if set.
func Register() ([]mcp.ToolDesc, map[string]mcp.ToolHandler) {
	cfg := LoadServerConfig()
	handlers := map[string]mcp.ToolHandler{}
	var descs []mcp.ToolDesc

	add := func(name, desc string, props map[string]interface{}, required []string, h mcp.ToolHandler) {
		if !cfg.toolEnabled(name) {
			return
		}
		annotateRelated(name, props)
		input := schema(props, required)
		input["x-datumbridge-capabilities"] = []string{"magento." + name}
		if doc := registrydocs.Markdown(name); doc != "" {
			input["x-datumbridge-docs"] = doc
		}
		descs = append(descs, mcp.ToolDesc{
			Name:        name,
			Description: formatToolDescription(name, desc),
			InputSchema: input,
		})
		handlers[name] = h
	}

	registerProductTools(add)
	registerProductImportTemplateTools(add)
	registerProductImportTools(add)
	registerOrderTools(add)
	registerCustomerTools(add)
	registerInventoryTools(add)
	registerCategoryTools(add)
	registerInvoiceShipmentTools(add)
	registerStoreTools(add)
	registerPromotionTools(add)
	registerCreditMemoTools(add)
	registerCmsTools(add)
	registerProductMediaTools(add)
	registerCartTools(add)

	return descs, handlers
}
