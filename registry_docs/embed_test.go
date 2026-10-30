package registrydocs

import (
	"strings"
	"testing"
)

var allTools = []string{
	// Products (9)
	"magento_list_products",
	"magento_get_product",
	"magento_create_product",
	"magento_update_product",
	"magento_delete_product",
	"magento_list_product_types",
	"magento_get_product_import_template",
	"magento_import_create_products",
	"magento_import_update_products",
	// Orders (5)
	"magento_list_orders",
	"magento_get_order",
	"magento_add_order_comment",
	"magento_cancel_order",
	"magento_hold_order",
	// Customers (4)
	"magento_list_customers",
	"magento_get_customer",
	"magento_create_customer",
	"magento_update_customer",
	// Inventory (4)
	"magento_get_stock_item",
	"magento_update_stock_item",
	"magento_list_inventory_sources",
	"magento_get_source_items",
	// Categories (4)
	"magento_list_categories",
	"magento_get_category",
	"magento_list_category_products",
	"magento_assign_category_product",
	// Invoices & Shipments (3)
	"magento_list_invoices",
	"magento_get_invoice",
	"magento_create_shipment",
	// Store (2)
	"magento_list_store_views",
	"magento_list_store_configs",
	// Promotions & Sales Rules (3)
	"magento_list_sales_rules",
	"magento_get_sales_rule",
	"magento_delete_coupon",
	// Credit Memos / Refunds (3)
	"magento_list_credit_memos",
	"magento_get_credit_memo",
	"magento_create_credit_memo",
	// CMS Pages & Blocks (4)
	"magento_list_cms_pages",
	"magento_get_cms_page",
	"magento_list_cms_blocks",
	"magento_get_cms_block",
	// Product Media (2)
	"magento_list_product_media",
	"magento_get_product_media",
	// Cart / Quotes (3)
	"magento_create_empty_cart",
	"magento_get_cart",
	"magento_add_cart_item",
}

func TestMarkdown_AllToolsExist(t *testing.T) {
	for _, name := range allTools {
		doc := Markdown(name)
		if strings.TrimSpace(doc) == "" {
			t.Errorf("missing or empty embedded markdown for tool %q", name)
		}
		if !strings.HasPrefix(doc, "# "+name) {
			t.Errorf("doc for %q must start with '# %s'", name, name)
		}
		for _, section := range []string{"## Audience", "## Agent rules", "## Parameters", "## Success fields", "## Error codes", "## Cases"} {
			if !strings.Contains(doc, section) {
				t.Errorf("doc for %q missing section %q", name, section)
			}
		}
		if !strings.Contains(doc, "Related tool") {
			t.Errorf("doc for %q missing 'Related tool' in Parameters table", name)
		}
		if len(doc) > 24000 {
			t.Errorf("doc for %q exceeds max 24000 chars: %d", name, len(doc))
		}
	}
}

func TestMarkdown_AudienceSection(t *testing.T) {
	for _, name := range allTools {
		doc := Markdown(name)
		if !strings.Contains(doc, "## Audience") {
			t.Fatalf("tool %q missing ## Audience", name)
		}
		part := strings.Split(doc, "## Audience")[1]
		part = strings.Split(part, "\n## ")[0]

		lines := strings.Split(part, "\n")
		var rows []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "|") && !strings.HasPrefix(trimmed, "|--") && !strings.Contains(trimmed, "Who") {
				rows = append(rows, trimmed)
			}
		}
		if len(rows) != 1 {
			t.Fatalf("tool %q: expected 1 Audience row, got %d", name, len(rows))
		}
		cols := strings.Split(rows[0], "|")
		if len(cols) < 3 {
			t.Fatalf("tool %q: invalid Audience row: %q", name, rows[0])
		}
		who := strings.TrimSpace(cols[1])
		token := strings.TrimSpace(cols[2])

		switch who {
		case "anonymous", "external", "internal":
		default:
			t.Errorf("tool %q: invalid Audience Who: %q", name, who)
		}

		switch token {
		case "required", "none":
		default:
			t.Errorf("tool %q: invalid Audience Token: %q", name, token)
		}

		if who == "anonymous" && token != "none" {
			t.Errorf("tool %q: anonymous audience must use token 'none', got %q", name, token)
		}
		if who != "anonymous" && token != "required" {
			t.Errorf("tool %q: %s audience must use token 'required', got %q", name, who, token)
		}
	}
}

func TestMarkdown_SecurityAndErrors(t *testing.T) {
	if Markdown("non_existent_tool_xyz") != "" {
		t.Fatal("missing tool must return empty string")
	}
	if Markdown("../cmd/api/main") != "" {
		t.Fatal("path traversal with .. must return empty string")
	}
	if Markdown("subdir/tool") != "" {
		t.Fatal("path traversal with / must return empty string")
	}
}
