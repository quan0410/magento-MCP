package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleMagentoPublicSearchProducts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"products": {
					"total_count": 2,
					"items": [
						{"id": 1, "sku": "BLZ-01", "name": "Blazer Basic", "stock_status": "IN_STOCK"},
						{"id": 2, "sku": "BLZ-02", "name": "Blazer Tweed", "stock_status": "OUT_OF_STOCK"}
					]
				}
			}
		}`))
	}))
	defer srv.Close()

	payload, _ := json.Marshal(map[string]interface{}{
		"store_url": srv.URL,
		"search":    "blazer",
	})

	res := handleMagentoPublicSearchProducts(payload)
	if errText, ok := res["error_message"].(string); ok && errText != "" {
		t.Fatalf("unexpected tool error: %s", errText)
	}
	if isErr, _ := res["isError"].(bool); isErr {
		t.Fatalf("tool returned error: %+v", res)
	}
}

func TestHandleMagentoPublicGetProduct(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"products": {
					"items": [
						{
							"id": 100,
							"sku": "WF-SK-013",
							"name": "Chân váy midi Cleo",
							"stock_status": "IN_STOCK"
						}
					]
				}
			}
		}`))
	}))
	defer srv.Close()

	payload, _ := json.Marshal(map[string]interface{}{
		"store_url": srv.URL,
		"sku":       "WF-SK-013",
	})

	res := handleMagentoPublicGetProduct(payload)
	if errText, ok := res["error_message"].(string); ok && errText != "" {
		t.Fatalf("unexpected tool error: %s", errText)
	}
}

func TestHandleMagentoPublicGetOutfit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"products": {
					"items": [
						{
							"sku": "BLZ-01",
							"name": "Áo khoác Blazer",
							"crosssell_products": [
								{"id": 2, "sku": "PNT-01", "name": "Quần tây ống rộng", "stock_status": "IN_STOCK"},
								{"id": 3, "sku": "SHR-01", "name": "Áo sơ mi lụa", "stock_status": "IN_STOCK"}
							],
							"related_products": [
								{"id": 4, "sku": "BAG-01", "name": "Túi xách tay", "stock_status": "IN_STOCK"}
							],
							"upsell_products": [
								{"id": 5, "sku": "BLZ-02", "name": "Áo khoác blazer dạ", "stock_status": "IN_STOCK"}
							]
						}
					]
				}
			}
		}`))
	}))
	defer srv.Close()

	payload, _ := json.Marshal(map[string]interface{}{
		"store_url": srv.URL,
		"sku":       "BLZ-01",
		"type":      "crosssell",
	})

	res := handleMagentoPublicGetOutfit(payload)
	if errText, ok := res["error_message"].(string); ok && errText != "" {
		t.Fatalf("unexpected tool error: %s", errText)
	}
}

func TestHandleMagentoPublicGuestCart(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"createEmptyCart": "guest-cart-hash-12345"
			}
		}`))
	}))
	defer srv.Close()

	payload, _ := json.Marshal(map[string]interface{}{
		"store_url": srv.URL,
		"action":    "create",
	})

	res := handleMagentoPublicGuestCart(payload)
	if errText, ok := res["error_message"].(string); ok && errText != "" {
		t.Fatalf("unexpected tool error: %s", errText)
	}
}
