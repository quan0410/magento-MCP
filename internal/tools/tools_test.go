package tools_test

import (
	"encoding/json"
	"fmt"
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
	if len(descs) != 49 {
		t.Fatalf("expected full tool scope (49 tools), got %d", len(descs))
	}

	wantedTools := []string{
		// Products (11)
		"magento_list_products", "magento_get_product", "magento_create_product",
		"magento_update_product", "magento_delete_product", "magento_list_product_types",
		"magento_get_product_import_template", "magento_import_create_products", "magento_import_update_products",
		"magento_get_media_import_status", "magento_get_related_products",
		// Orders (6)
		"magento_list_orders", "magento_get_order", "magento_add_order_comment",
		"magento_cancel_order", "magento_hold_order", "magento_verify_and_get_order",
		// Customers (4)
		"magento_list_customers", "magento_get_customer", "magento_create_customer",
		"magento_update_customer",
		// Inventory (4)
		"magento_get_stock_item", "magento_update_stock_item", "magento_list_inventory_sources",
		"magento_get_source_items",
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

func getToolResultText(res map[string]interface{}) string {
	switch c := res["content"].(type) {
	case []map[string]string:
		if len(c) > 0 {
			return c[0]["text"]
		}
	case []interface{}:
		if len(c) > 0 {
			if m, ok := c[0].(map[string]interface{}); ok {
				return fmt.Sprint(m["text"])
			}
		}
	}
	return ""
}

func TestHandleMagentoVerifyAndGetOrder(t *testing.T) {
	clearToolFilterEnv(t)
	orderJSON := `{"items":[{"increment_id":"100000025","customer_email":"customer@example.com","status":"processing","created_at":"2026-10-25 10:00:00","grand_total":550000,"order_currency_code":"VND","billing_address":{"firstname":"Van","lastname":"Nguyen","telephone":"0987654321","city":"Ho Chi Minh","region":"District 1"},"items":[{"name":"Ao so mi trang","sku":"SHIRT-WHT-M","qty_ordered":1,"price":550000,"row_total":550000}]}],"total_count":1}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/V1/orders" {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(orderJSON))
	}))
	defer srv.Close()

	args := testCreds(t, srv)
	_, handlers := tools.Register()

	// 1. Success with email
	{
		var m map[string]interface{}
		_ = json.Unmarshal(args, &m)
		m["order_number"] = "#100000025"
		m["verification"] = "customer@example.com"
		raw, _ := json.Marshal(m)

		res := handlers["magento_verify_and_get_order"](raw)
		if res["isError"] == true {
			t.Fatalf("unexpected error: %+v", res)
		}
		text := getToolResultText(res)
		if !strings.Contains(text, `"verified": true`) && !strings.Contains(text, `"verified":true`) {
			t.Fatalf("expected verified=true, got: %s", text)
		}
		if !strings.Contains(text, "SHIRT-WHT-M") {
			t.Fatalf("expected item SKU SHIRT-WHT-M, got: %s", text)
		}
	}

	// 2. Success with phone
	{
		var m map[string]interface{}
		_ = json.Unmarshal(args, &m)
		m["order_number"] = "100000025"
		m["verification"] = "+84987654321"
		raw, _ := json.Marshal(m)

		res := handlers["magento_verify_and_get_order"](raw)
		if res["isError"] == true {
			t.Fatalf("unexpected error: %+v", res)
		}
		text := getToolResultText(res)
		if !strings.Contains(text, `"verified": true`) && !strings.Contains(text, `"verified":true`) {
			t.Fatalf("expected verified=true with phone, got: %s", text)
		}
	}

	// 3. Mismatch verification
	{
		var m map[string]interface{}
		_ = json.Unmarshal(args, &m)
		m["order_number"] = "100000025"
		m["verification"] = "wrong@example.com"
		raw, _ := json.Marshal(m)

		res := handlers["magento_verify_and_get_order"](raw)
		if res["isError"] == true {
			t.Fatalf("unexpected error: %+v", res)
		}
		text := getToolResultText(res)
		if !strings.Contains(text, `"verified": false`) && !strings.Contains(text, `"verified":false`) {
			t.Fatalf("expected verified=false, got: %s", text)
		}
	}
}

func TestHandleMagentoGetRelatedProducts(t *testing.T) {
	clearToolFilterEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.URL.Path == "/rest/V1/products/SHIRT-01/links/crosssell" {
			_, _ = w.Write([]byte(`[{"sku":"SHIRT-01","link_type":"crosssell","linked_product_sku":"PANT-01"}]`))
			return
		}
		if r.URL.Path == "/rest/V1/products" {
			_, _ = w.Write([]byte(`{"items":[{"sku":"PANT-01","name":"Quan tay cong so","price":450000,"status":1,"type_id":"simple"}],"total_count":1}`))
			return
		}
		t.Errorf("unexpected path: %q", r.URL.Path)
	}))
	defer srv.Close()

	args := testCreds(t, srv)
	_, handlers := tools.Register()

	var m map[string]interface{}
	_ = json.Unmarshal(args, &m)
	m["sku"] = "SHIRT-01"
	m["link_type"] = "crosssell"
	raw, _ := json.Marshal(m)

	res := handlers["magento_get_related_products"](raw)
	if res["isError"] == true {
		t.Fatalf("unexpected error: %+v", res)
	}
	text := getToolResultText(res)
	if !strings.Contains(text, "PANT-01") || !strings.Contains(text, "Quan tay cong so") {
		t.Fatalf("expected PANT-01 in result, got: %s", text)
	}
}

func TestHandleGetSourceItemsSuccess(t *testing.T) {
	clearToolFilterEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/V1/inventory/source-items" {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		q := r.URL.Query()
		skuFound := false
		sourceFound := false
		for i := 0; i < 5; i++ {
			field := q.Get(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][field]", i))
			val := q.Get(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][value]", i))
			if field == "sku" && val == "BLZ-01" {
				skuFound = true
			}
			if field == "source_code" && val == "vincom_dk" {
				sourceFound = true
			}
		}
		if !skuFound {
			t.Errorf("expected sku filter BLZ-01, got query: %v", q)
		}
		if !sourceFound {
			t.Errorf("expected source_code filter vincom_dk, got query: %v", q)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[{"sku":"BLZ-01","source_code":"vincom_dk","quantity":3,"status":1}],"total_count":1}`))
	}))
	defer srv.Close()

	args := testCreds(t, srv)
	var m map[string]interface{}
	_ = json.Unmarshal(args, &m)
	m["sku"] = "BLZ-01"
	m["source_code"] = "vincom_dk"
	raw, _ := json.Marshal(m)

	_, handlers := tools.Register()
	res := handlers["magento_get_source_items"](raw)
	if res["isError"] == true {
		t.Fatalf("unexpected error: %+v", res)
	}
	text := getToolResultText(res)
	if !strings.Contains(text, "vincom_dk") || !strings.Contains(text, `"quantity": 3`) {
		t.Fatalf("expected source item details in result, got: %s", text)
	}
}

func TestHandleGetSourceItemsMissingSKU(t *testing.T) {
	clearToolFilterEnv(t)
	_, handlers := tools.Register()
	res := handlers["magento_get_source_items"](json.RawMessage(`{"credentials_json":"{\"base_url\":\"https://x.com\",\"consumer_key\":\"k\",\"consumer_secret\":\"s\",\"access_token\":\"t\",\"access_token_secret\":\"ts\"}"}`))
	if res["isError"] != true {
		t.Fatalf("expected error result for missing sku, got: %+v", res)
	}
}

func TestHandleListProductsWithCategoryID(t *testing.T) {
	clearToolFilterEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/V1/products" {
			t.Errorf("unexpected path: %q", r.URL.Path)
		}
		q := r.URL.Query()
		categoryFound := false
		for i := 0; i < 5; i++ {
			field := q.Get(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][field]", i))
			val := q.Get(fmt.Sprintf("searchCriteria[filterGroups][%d][filters][0][value]", i))
			if field == "category_id" && val == "15" {
				categoryFound = true
			}
		}
		if !categoryFound {
			t.Errorf("expected category_id filter 15, got query: %v", q)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[{"sku":"DRESS-01","name":"Vay da hoi den","price":1500000}],"total_count":1}`))
	}))
	defer srv.Close()

	args := testCreds(t, srv)
	var m map[string]interface{}
	_ = json.Unmarshal(args, &m)
	m["category_id"] = "15"
	raw, _ := json.Marshal(m)

	_, handlers := tools.Register()
	res := handlers["magento_list_products"](raw)
	if res["isError"] == true {
		t.Fatalf("unexpected error: %+v", res)
	}
	text := getToolResultText(res)
	if !strings.Contains(text, "DRESS-01") {
		t.Fatalf("expected DRESS-01 in result, got: %s", text)
	}
}
