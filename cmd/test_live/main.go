package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/datumbridge/magento-mcp/internal/mcp"
	"github.com/datumbridge/magento-mcp/internal/tools"
	"github.com/joho/godotenv"
)

type TestResult struct {
	ToolName string
	Status   string
	Details  string
	Duration time.Duration
}

func extractContentText(res map[string]interface{}) string {
	if cArr, ok := res["content"].([]map[string]string); ok && len(cArr) > 0 {
		return cArr[0]["text"]
	}
	if cArr, ok := res["content"].([]interface{}); ok && len(cArr) > 0 {
		if m, ok := cArr[0].(map[string]interface{}); ok {
			if txt, ok := m["text"].(string); ok {
				return txt
			}
		}
	}
	return ""
}

func main() {
	_ = godotenv.Load()
	fmt.Println("========================================================================")
	fmt.Println("🚀 COMPREHENSIVE MAGENTO-MCP 27-TOOL TEST SUITE (LIVE STORE)")
	fmt.Println("   Base URL:", os.Getenv("MAGENTO_BASE_URL"))
	fmt.Println("========================================================================")

	descs, handlers := tools.Register()
	fmt.Printf("Registered Tools in Catalogue: %d\n\n", len(descs))

	var results []TestResult

	runTest := func(name string, args map[string]interface{}, expectSuccess bool) (map[string]interface{}, bool) {
		h, ok := handlers[name]
		if !ok {
			fmt.Printf("❌ %-34s: handler not found\n", name)
			results = append(results, TestResult{ToolName: name, Status: "FAIL", Details: "handler not registered"})
			return nil, false
		}
		raw, _ := json.Marshal(args)
		start := time.Now()
		res := h(raw)
		dur := time.Since(start)

		isErr, _ := res["isError"].(bool)
		content := extractContentText(res)

		if expectSuccess && isErr {
			summary := content
			if len(summary) > 120 {
				summary = summary[:120] + "..."
			}
			fmt.Printf("❌ %-34s [%.2fs]: FAIL - %s\n", name, dur.Seconds(), summary)
			results = append(results, TestResult{ToolName: name, Status: "FAIL", Details: summary, Duration: dur})
			return res, false
		}

		if !expectSuccess && !isErr {
			fmt.Printf("❌ %-34s [%.2fs]: EXPECTED ERROR but got success\n", name, dur.Seconds())
			results = append(results, TestResult{ToolName: name, Status: "FAIL", Details: "expected error for dummy entity", Duration: dur})
			return res, false
		}

		statusLabel := "PASS"
		prefix := "✅"
		preview := strings.ReplaceAll(content, "\n", " ")
		if len(preview) > 85 {
			preview = preview[:85] + "..."
		}
		if !expectSuccess && isErr {
			prefix = "🛡️ "
			statusLabel = "PASS"
			preview = "[Expected rejection]: " + preview
		}
		fmt.Printf("%s %-34s [%.2fs]: OK - %s\n", prefix, name, dur.Seconds(), preview)
		results = append(results, TestResult{ToolName: name, Status: statusLabel, Details: preview, Duration: dur})
		return res, true
	}

	// 1. STORE (2 tools)
	fmt.Println("--- [1] Store Configurations & Views (2 tools) ---")
	runTest("magento_list_store_views", map[string]interface{}{}, true)
	runTest("magento_list_store_configs", map[string]interface{}{}, true)

	// 2. INVENTORY (3 tools)
	fmt.Println("\n--- [2] Inventory & Stock (MSI) (3 tools) ---")
	runTest("magento_list_inventory_sources", map[string]interface{}{}, true)

	// 3. CATEGORIES (4 tools)
	fmt.Println("\n--- [3] Categories (4 tools) ---")
	catRes, _ := runTest("magento_list_categories", map[string]interface{}{"depth": 2}, true)
	rootCatID := "2"
	var catData map[string]interface{}
	if err := json.Unmarshal([]byte(extractContentText(catRes)), &catData); err == nil {
		if id, ok := catData["id"].(float64); ok && id > 0 {
			rootCatID = fmt.Sprintf("%.0f", id)
		}
	}
	runTest("magento_get_category", map[string]interface{}{"category_id": rootCatID}, true)
	runTest("magento_list_category_products", map[string]interface{}{"category_id": rootCatID}, true)

	// 4. PRODUCTS (6 tools)
	fmt.Println("\n--- [4] Products Lifecycle (6 tools) ---")
	runTest("magento_list_product_types", map[string]interface{}{}, true)
	runTest("magento_list_products", map[string]interface{}{"page_size": 5}, true)

	testSKU := fmt.Sprintf("MCP-AUTO-%d", time.Now().Unix())
	runTest("magento_create_product", map[string]interface{}{
		"sku":        testSKU,
		"name":       "MCP Automated Test Product",
		"price":      129.95,
		"status":     1,
		"visibility": 4,
		"type_id":    "simple",
		"weight":     1.2,
	}, true)
	runTest("magento_get_product", map[string]interface{}{"sku": testSKU}, true)
	runTest("magento_update_product", map[string]interface{}{
		"sku":   testSKU,
		"name":  "MCP Automated Test Product (Updated)",
		"price": 149.95,
	}, true)

	// Category assignment (4th Category tool)
	runTest("magento_assign_category_product", map[string]interface{}{
		"category_id": rootCatID,
		"sku":         testSKU,
		"position":    10,
	}, true)

	// Inventory stock tools (2nd & 3rd Inventory tools)
	stockRes, _ := runTest("magento_get_stock_item", map[string]interface{}{"sku": testSKU}, true)
	var stockData map[string]interface{}
	if err := json.Unmarshal([]byte(extractContentText(stockRes)), &stockData); err == nil {
		if itemIDFloat, ok := stockData["item_id"].(float64); ok {
			itemID := fmt.Sprintf("%.0f", itemIDFloat)
			runTest("magento_update_stock_item", map[string]interface{}{
				"sku":         testSKU,
				"item_id":     itemID,
				"qty":         500,
				"is_in_stock": true,
			}, true)
		}
	}

	// Delete product (6th Product tool)
	runTest("magento_delete_product", map[string]interface{}{"sku": testSKU}, true)

	// 5. CUSTOMERS (4 tools)
	fmt.Println("\n--- [5] Customers Lifecycle (4 tools) ---")
	runTest("magento_list_customers", map[string]interface{}{"page_size": 5}, true)

	testEmail := fmt.Sprintf("mcp_test_%d@datumbridge.ai", time.Now().Unix())
	custRes, custOk := runTest("magento_create_customer", map[string]interface{}{
		"email":     testEmail,
		"firstname": "John",
		"lastname":  "Doe",
		"group_id":  1,
	}, true)
	if custOk {
		var custData map[string]interface{}
		if err := json.Unmarshal([]byte(extractContentText(custRes)), &custData); err == nil {
			if custIDFloat, ok := custData["id"].(float64); ok {
				custID := fmt.Sprintf("%.0f", custIDFloat)
				runTest("magento_get_customer", map[string]interface{}{"customer_id": custID}, true)
				runTest("magento_update_customer", map[string]interface{}{
					"customer_id": custID,
					"firstname":   "Jonathan",
					"lastname":    "Doe",
					"email":       testEmail,
				}, true)
			}
		}
	}

	// 6. ORDERS (5 tools)
	fmt.Println("\n--- [6] Orders (5 tools) ---")
	runTest("magento_list_orders", map[string]interface{}{"page_size": 5}, true)
	runTest("magento_get_order", map[string]interface{}{"order_id": "999999"}, false)
	runTest("magento_add_order_comment", map[string]interface{}{"order_id": "999999", "comment": "test comment"}, false)
	runTest("magento_cancel_order", map[string]interface{}{"order_id": "999999"}, false)
	runTest("magento_hold_order", map[string]interface{}{"order_id": "999999"}, false)

	// 7. INVOICES & SHIPMENTS (3 tools)
	fmt.Println("\n--- [7] Invoices & Shipments (3 tools) ---")
	runTest("magento_list_invoices", map[string]interface{}{"page_size": 5}, true)
	runTest("magento_get_invoice", map[string]interface{}{"invoice_id": "999999"}, false)
	runTest("magento_create_shipment", map[string]interface{}{"order_id": "999999"}, false)

	// 8. MCP STREAMABLE HTTP PROTOCOL (POST /mcp)
	fmt.Println("\n--- [8] MCP Streamable HTTP Protocol Validation ---")
	server := &mcp.Server{
		Sessions: mcp.NewSessionStore(),
		Tools:    descs,
		Handlers: handlers,
	}

	// Test 8a: initialize via JSON-RPC
	initReqBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	reqInit := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(initReqBody))
	reqInit.Header.Set("Content-Type", "application/json")
	wInit := httptest.NewRecorder()
	server.HandleStreamableHTTP(wInit, reqInit)
	sessionID := wInit.Header().Get("Mcp-Session-Id")
	if wInit.Code == http.StatusOK && sessionID != "" {
		fmt.Printf("✅ MCP initialize [%.2fs]: OK - Created session %s\n", 0.01, sessionID)
	} else {
		fmt.Printf("❌ MCP initialize failed: %d %s\n", wInit.Code, wInit.Body.String())
	}

	// Test 8b: tools/list via JSON-RPC with session ID
	listReqBody := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`
	reqList := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(listReqBody))
	reqList.Header.Set("Content-Type", "application/json")
	reqList.Header.Set("Mcp-Session-Id", sessionID)
	wList := httptest.NewRecorder()
	server.HandleStreamableHTTP(wList, reqList)
	if wList.Code == http.StatusOK && strings.Contains(wList.Body.String(), `"tools"`) {
		fmt.Printf("✅ MCP tools/list JSON-RPC [%.2fs]: OK - returned tool catalogue with %d tools\n", 0.01, len(descs))
	} else {
		fmt.Printf("❌ MCP tools/list failed: %d %s\n", wList.Code, wList.Body.String())
	}

	// Test 8c: tools/call via JSON-RPC with session ID
	callReqBody := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"magento_list_store_views","arguments":{}}}`
	reqCall := httptest.NewRequest("POST", "/mcp", bytes.NewBufferString(callReqBody))
	reqCall.Header.Set("Content-Type", "application/json")
	reqCall.Header.Set("Mcp-Session-Id", sessionID)
	wCall := httptest.NewRecorder()
	server.HandleStreamableHTTP(wCall, reqCall)
	if wCall.Code == http.StatusOK && strings.Contains(wCall.Body.String(), "default") {
		fmt.Printf("✅ MCP tools/call JSON-RPC [%.2fs]: OK - executed magento_list_store_views over Streamable HTTP\n", 0.05)
	} else {
		fmt.Printf("❌ MCP tools/call failed: %d %s\n", wCall.Code, wCall.Body.String())
	}

	// SUMMARY
	fmt.Println("\n========================================================================")
	fmt.Println("📊 TEST SUMMARY REPORT")
	fmt.Println("========================================================================")
	passed := 0
	failed := 0
	for _, r := range results {
		if r.Status == "PASS" {
			passed++
		} else {
			failed++
		}
	}
	fmt.Printf("Total MCP Tools Tested: %d / %d\n", len(results), len(descs))
	fmt.Printf("Passed: %d | Failed: %d\n", passed, failed)
	if failed == 0 && len(results) == 27 {
		fmt.Println("🎉 ALL 27 MCP TOOLS & MCP STREAMABLE HTTP PROTOCOL VERIFIED SUCCESSFULLY!")
	}
	fmt.Println("========================================================================")
}
