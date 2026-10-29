package tools

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func createTestXLSX(t *testing.T, headers []string, rows [][]interface{}) []byte {
	t.Helper()
	f := excelize.NewFile()
	sheet := "Products"
	f.SetSheetName("Sheet1", sheet)

	for colIdx, colName := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheet, cell, colName)
	}

	for rowIdx, row := range rows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheet, cell, val)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("failed to write test xlsx: %v", err)
	}
	return buf.Bytes()
}

func TestImportParser_XLSX(t *testing.T) {
	headers := []string{"sku", "name", "price", "qty", "status", "brand", "color"}
	rowsData := [][]interface{}{
		{"TSHIRT-01", "Cool T-Shirt", 199.99, 50, 1, "Nike", "Blue"},
		{"TSHIRT-02", "Warm T-Shirt", 299.00, 20, "enabled", "Adidas", "Red"},
	}
	xlsxBytes := createTestXLSX(t, headers, rowsData)

	rows, err := parseProductsFile(xlsxBytes, ".xlsx")
	if err != nil {
		t.Fatalf("parseProductsFile failed on xlsx: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	r1 := rows[0]
	if r1.SKU != "TSHIRT-01" || r1.Name != "Cool T-Shirt" || r1.Price == nil || *r1.Price != 199.99 {
		t.Fatalf("row 1 parsed incorrectly: %+v", r1)
	}
	if r1.Qty == nil || *r1.Qty != 50 {
		t.Fatalf("expected qty 50, got %v", r1.Qty)
	}
	if r1.Status == nil || *r1.Status != 1 {
		t.Fatalf("expected status 1, got %v", r1.Status)
	}
	if r1.CustomAttributes["brand"] != "Nike" || r1.CustomAttributes["color"] != "Blue" {
		t.Fatalf("dynamic custom attributes parsed incorrectly: %+v", r1.CustomAttributes)
	}

	r2 := rows[1]
	if r2.Status == nil || *r2.Status != 1 {
		t.Fatalf("expected status 'enabled' to map to 1, got %v", r2.Status)
	}
}

func TestImportParser_RejectCSV(t *testing.T) {
	csvData := []byte("sku,name,price\nPROD-1,Test,10\n")
	_, err := parseProductsFile(csvData, ".csv")
	if err == nil {
		t.Fatal("expected error rejecting CSV file, got nil")
	}
	if !strings.Contains(err.Error(), "only Excel (.xlsx) files are supported") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestGetProductImportTemplate_XLSX(t *testing.T) {
	args, _ := json.Marshal(map[string]interface{}{
		"type": "create",
	})
	res := handleMagentoGetProductImportTemplate(args)
	if res["success"] != true {
		t.Fatalf("expected success true, got %v", res)
	}
	if res["format"] != "xlsx" {
		t.Fatalf("expected format 'xlsx', got %v", res["format"])
	}
	b64, ok := res["file_base64"].(string)
	if !ok || len(b64) == 0 {
		t.Fatalf("expected non-empty file_base64, got %v", res["file_base64"])
	}
	guide, ok := res["fields_guide"].(map[string]interface{})
	if !ok || len(guide) == 0 {
		t.Fatalf("expected non-empty fields_guide, got %v", res["fields_guide"])
	}
}

func TestImportCreateProducts_LocalXLSX(t *testing.T) {
	// Mock Magento API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/V1/products/") {
			// Return 404 so product is considered new
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message": "The product that was requested doesn't exist."}`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/rest/V1/products" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 101, "sku": "NEW-PROD-01", "name": "Test Product", "price": 100}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	// Write temp XLSX
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "products.xlsx")
	xlsxBytes := createTestXLSX(t, []string{"sku", "name", "price", "qty"}, [][]interface{}{
		{"NEW-PROD-01", "Test Product", 100.0, 10},
	})
	if err := os.WriteFile(xlsxPath, xlsxBytes, 0644); err != nil {
		t.Fatalf("failed to write temp XLSX: %v", err)
	}

	credsJSON, _ := json.Marshal(map[string]string{
		"base_url":            server.URL,
		"consumer_key":        "ck",
		"consumer_secret":     "cs",
		"access_token":        "at",
		"access_token_secret": "ats",
	})

	args, _ := json.Marshal(map[string]interface{}{
		"credentials_json": string(credsJSON),
		"file_path":        xlsxPath,
	})

	res := handleMagentoImportCreateProducts(args)
	if res["success"] != true {
		t.Fatalf("expected success true, got %+v", res)
	}
	createdCount, ok := res["created_count"].(int)
	if !ok || createdCount != 1 {
		t.Fatalf("expected created_count 1, got %v", res["created_count"])
	}
}

func TestImportUpdateProducts_LocalXLSX(t *testing.T) {
	// Mock Magento API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/rest/V1/products/") {
			// Return 200 so product is found
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 101, "sku": "EXISTING-01", "name": "Old Name", "price": 100}`))
			return
		}
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/rest/V1/products/") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 101, "sku": "EXISTING-01", "name": "Updated Name", "price": 150}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	// Write temp XLSX
	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "update_products.xlsx")
	xlsxBytes := createTestXLSX(t, []string{"sku", "price"}, [][]interface{}{
		{"EXISTING-01", 150.0},
	})
	if err := os.WriteFile(xlsxPath, xlsxBytes, 0644); err != nil {
		t.Fatalf("failed to write temp XLSX: %v", err)
	}

	credsJSON, _ := json.Marshal(map[string]string{
		"base_url":            server.URL,
		"consumer_key":        "ck",
		"consumer_secret":     "cs",
		"access_token":        "at",
		"access_token_secret": "ats",
	})

	args, _ := json.Marshal(map[string]interface{}{
		"credentials_json": string(credsJSON),
		"file_path":        xlsxPath,
	})

	res := handleMagentoImportUpdateProducts(args)
	if res["success"] != true {
		t.Fatalf("expected success true, got %+v", res)
	}
	updatedCount, ok := res["updated_count"].(int)
	if !ok || updatedCount != 1 {
		t.Fatalf("expected updated_count 1, got %v", res["updated_count"])
	}
}

func TestNormalizeImportDate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"12-31-26", "2026-12-31"},
		{"2026-12-31 00:00:00", "2026-12-31"},
		{"2026-12-31", "2026-12-31"},
		{"12/31/2026", "2026-12-31"},
		{"46327", "2026-11-01"},
		{"", ""},
	}

	for _, tt := range tests {
		actual := normalizeImportDate(tt.input)
		if actual != tt.expected {
			t.Errorf("normalizeImportDate(%q) = %q, expected %q", tt.input, actual, tt.expected)
		}
	}
}

func TestImportCreateProducts_BatchingPagination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items": []}`))
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	xlsxPath := filepath.Join(tmpDir, "batch_test.xlsx")
	xlsxBytes := createTestXLSX(t, []string{"sku", "name", "price"}, [][]interface{}{
		{"SKU-1", "Prod 1", 10.0},
		{"SKU-2", "Prod 2", 20.0},
		{"SKU-3", "Prod 3", 30.0},
		{"SKU-4", "Prod 4", 40.0},
		{"SKU-5", "Prod 5", 50.0},
	})
	if err := os.WriteFile(xlsxPath, xlsxBytes, 0644); err != nil {
		t.Fatalf("failed to write test xlsx: %v", err)
	}

	credsJSON, _ := json.Marshal(map[string]string{
		"base_url":            server.URL,
		"consumer_key":        "ck",
		"consumer_secret":     "cs",
		"access_token":        "at",
		"access_token_secret": "ats",
	})

	// Batch 1: offset 0, limit 2
	args1, _ := json.Marshal(map[string]interface{}{
		"credentials_json": string(credsJSON),
		"file_path":        xlsxPath,
		"offset":           0,
		"limit":            2,
	})
	res1 := handleMagentoImportCreateProducts(args1)
	if res1["processed_count"] != 2 || res1["has_more"] != true || res1["next_offset"] != 2 {
		t.Fatalf("unexpected batch 1 result: %+v", res1)
	}

	// Batch 2: offset 2, limit 2
	args2, _ := json.Marshal(map[string]interface{}{
		"credentials_json": string(credsJSON),
		"file_path":        xlsxPath,
		"offset":           2,
		"limit":            2,
	})
	res2 := handleMagentoImportCreateProducts(args2)
	if res2["processed_count"] != 2 || res2["has_more"] != true || res2["next_offset"] != 4 {
		t.Fatalf("unexpected batch 2 result: %+v", res2)
	}

	// Batch 3: offset 4, limit 2 (only 1 remaining)
	args3, _ := json.Marshal(map[string]interface{}{
		"credentials_json": string(credsJSON),
		"file_path":        xlsxPath,
		"offset":           4,
		"limit":            2,
	})
	res3 := handleMagentoImportCreateProducts(args3)
	if res3["processed_count"] != 1 || res3["has_more"] != false || res3["next_offset"] != 0 {
		t.Fatalf("unexpected batch 3 result: %+v", res3)
	}
}


