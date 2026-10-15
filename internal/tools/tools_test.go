package tools_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/datumbridge/magento-mcp/internal/tools"
)

func clearToolFilterEnv(t *testing.T) {
	t.Helper()
	t.Setenv("MAGENTO_TOOLS", "")
}

func TestRegisterFullScopeByDefault(t *testing.T) {
	clearToolFilterEnv(t)
	descs, handlers := tools.Register()
	if len(descs) != 42 {
		t.Fatalf("expected full tool scope (42 tools), got %d", len(descs))
	}

	wantedTools := []string{
		// Products (6)
		"magento_list_products", "magento_get_product", "magento_create_product",
		"magento_update_product", "magento_delete_product", "magento_list_product_types",
		// Orders (5)
		"magento_list_orders", "magento_get_order", "magento_add_order_comment",
		"magento_cancel_order", "magento_hold_order",
		// Customers (4)
		"magento_list_customers", "magento_get_customer", "magento_create_customer",
		"magento_update_customer",
		// Inventory (3)
		"magento_get_stock_item", "magento_update_stock_item", "magento_list_inventory_sources",
		// Categories (4)
		"magento_list_categories", "magento_get_category", "magento_list_category_products",
		"magento_assign_category_product",
		// Invoices & Shipments (3)
		"magento_list_invoices", "magento_get_invoice", "magento_create_shipment",
		// Store (2)
		"magento_list_store_views", "magento_list_store_configs",
		// Promotions & Sales Rules (3)
		"magento_list_sales_rules", "magento_get_sales_rule", "magento_delete_coupon",
		// Credit Memos / Refunds (3)
		"magento_list_credit_memos", "magento_get_credit_memo", "magento_create_credit_memo",
		// CMS (4)
		"magento_list_cms_pages", "magento_get_cms_page", "magento_list_cms_blocks", "magento_get_cms_block",
		// Product Media (2)
		"magento_list_product_media", "magento_get_product_media",
		// Cart (3)
		"magento_create_empty_cart", "magento_get_cart", "magento_add_cart_item",
	}

	for _, want := range wantedTools {
		if _, ok := handlers[want]; !ok {
			t.Fatalf("missing tool handler: %s", want)
		}
	}

	for _, d := range descs {
		doc, ok := d.InputSchema["x-datumbridge-docs"].(string)
		if !ok || doc == "" {
			t.Errorf("tool %s missing x-datumbridge-docs in InputSchema", d.Name)
		}
		caps, ok := d.InputSchema["x-datumbridge-capabilities"].([]string)
		if !ok || len(caps) == 0 {
			t.Errorf("tool %s missing x-datumbridge-capabilities in InputSchema", d.Name)
		}
		if !strings.Contains(d.Description, "Capabilities: magento."+d.Name) {
			t.Errorf("tool %s description missing Capabilities: magento.%s", d.Name, d.Name)
		}
		if !strings.Contains(d.Description, "Outputs:") {
			t.Errorf("tool %s description missing Outputs:", d.Name)
		}
	}
}

func TestRegisterCustomAllowlist(t *testing.T) {
	clearToolFilterEnv(t)
	t.Setenv("MAGENTO_TOOLS", "magento_get_product,magento_list_orders")
	descs, handlers := tools.Register()
	if len(descs) != 2 {
		t.Fatalf("expected 2 allowlisted tools, got %d", len(descs))
	}
	if _, ok := handlers["magento_get_product"]; !ok {
		t.Fatal("magento_get_product should be enabled")
	}
	if _, ok := handlers["magento_list_orders"]; !ok {
		t.Fatal("magento_list_orders should be enabled")
	}
	if _, ok := handlers["magento_create_product"]; ok {
		t.Fatal("magento_create_product should be filtered out")
	}
}

func testCreds(t *testing.T, srv *httptest.Server) json.RawMessage {
	t.Helper()
	t.Setenv("MAGENTO_API_BASE", srv.URL)
	creds := map[string]string{
		"base_url":            srv.URL,
		"consumer_key":        "mock_ck",
		"consumer_secret":     "mock_cs",
		"access_token":        "mock_at",
		"access_token_secret": "mock_ats",
	}
	bundleJSON, _ := json.Marshal(creds)
	raw, _ := json.Marshal(map[string]string{"credentials_json": string(bundleJSON)})
	return raw
}

func TestHandleGetProductSuccess(t *testing.T) {
	clearToolFilterEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "OAuth ") {
			t.Errorf("missing or invalid OAuth header: %q", auth)
		}
		if r.URL.Path != "/rest/V1/products/24-MB01" {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"sku":"24-MB01","name":"Joust Duffle Bag","price":34.00}`))
	}))
	defer srv.Close()

	args := testCreds(t, srv)
	var m map[string]interface{}
	_ = json.Unmarshal(args, &m)
	m["sku"] = "24-MB01"
	raw, _ := json.Marshal(m)

	_, handlers := tools.Register()
	res := handlers["magento_get_product"](raw)
	if res["isError"] == true {
		t.Fatalf("unexpected error result: %+v", res)
	}
}

func TestHandleGetProductMissingSKU(t *testing.T) {
	clearToolFilterEnv(t)
	_, handlers := tools.Register()
	res := handlers["magento_get_product"](json.RawMessage(`{"credentials_json":"{\"base_url\":\"https://x.com\",\"consumer_key\":\"k\",\"consumer_secret\":\"s\",\"access_token\":\"t\",\"access_token_secret\":\"ts\"}"}`))
	if res["isError"] != true {
		t.Fatalf("expected error result for missing sku, got: %+v", res)
	}
}

func TestHandleCreateProductSuccess(t *testing.T) {
	clearToolFilterEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		prod := body["product"].(map[string]interface{})
		if prod["sku"] != "SKU-99" {
			t.Errorf("expected SKU-99, got %v", prod["sku"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"sku":"SKU-99","name":"Test Product"}`))
	}))
	defer srv.Close()

	args := testCreds(t, srv)
	var m map[string]interface{}
	_ = json.Unmarshal(args, &m)
	m["sku"] = "SKU-99"
	m["name"] = "Test Product"
	m["price"] = 99.95
	raw, _ := json.Marshal(m)

	_, handlers := tools.Register()
	res := handlers["magento_create_product"](raw)
	if res["isError"] == true {
		t.Fatalf("unexpected error result: %+v", res)
	}
}

func TestHandleListOrdersSuccess(t *testing.T) {
	clearToolFilterEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/V1/orders" {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[{"entity_id":1,"status":"pending"}]}`))
	}))
	defer srv.Close()

	args := testCreds(t, srv)
	_, handlers := tools.Register()
	res := handlers["magento_list_orders"](args)
	if res["isError"] == true {
		t.Fatalf("unexpected error result: %+v", res)
	}
}

func TestHandleListStoreViewsSuccess(t *testing.T) {
	clearToolFilterEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/V1/store/storeViews" {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"id":1,"code":"default"}]`))
	}))
	defer srv.Close()

	args := testCreds(t, srv)
	_, handlers := tools.Register()
	res := handlers["magento_list_store_views"](args)
	if res["isError"] == true {
		t.Fatalf("unexpected error result: %+v", res)
	}
}
