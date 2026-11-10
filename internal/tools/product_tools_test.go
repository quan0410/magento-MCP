package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnrichConfigurableProduct_WithParentContextFallback(t *testing.T) {
	// Recreating the exact scenario from user's JSON:
	// Parent product has 3 images (without labels) and 3 colors (Nâu latte, Xanh denim bụi, Xanh olive).
	// url_key and description specify "Xanh olive".
	prod := map[string]interface{}{
		"id":        float64(1636),
		"sku":       "WF-SK-013",
		"name":      "Chân váy midi phối đai Cleo",
		"type_id":   "configurable",
		"url_key":   "chan-vay-midi-phoi-dai-cleo-xanh-olive-wf-sk-013",
		"image_url": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-01_1.jpg",
		"media_gallery_entries": []interface{}{
			map[string]interface{}{
				"id":    float64(3631),
				"file":  "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-01_1.jpg",
				"url":   "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-01_1.jpg",
				"label": "",
			},
			map[string]interface{}{
				"id":    float64(3632),
				"file":  "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-02_1.jpg",
				"url":   "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-02_1.jpg",
				"label": "",
			},
			map[string]interface{}{
				"id":    float64(3633),
				"file":  "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-03_1.jpg",
				"url":   "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-03_1.jpg",
				"label": "",
			},
		},
		"custom_attributes": []interface{}{
			map[string]interface{}{
				"attribute_code": "url_key",
				"value":          "chan-vay-midi-phoi-dai-cleo-xanh-olive-wf-sk-013",
			},
			map[string]interface{}{
				"attribute_code": "description",
				"value":          "<p>Chân váy midi phối đai Cleo mang sắc xanh olive...</p>",
			},
		},
		"extension_attributes": map[string]interface{}{
			"configurable_product_options": []interface{}{
				map[string]interface{}{
					"attribute_id": "93",
					"id":           float64(244),
					"label":        "Màu sắc",
					"values": []interface{}{
						map[string]interface{}{
							"label":       "Nâu latte",
							"value_index": float64(126),
						},
						map[string]interface{}{
							"label":       "Xanh denim bụi",
							"value_index": float64(136),
						},
						map[string]interface{}{
							"label":       "Xanh olive",
							"value_index": float64(138),
						},
					},
				},
			},
		},
	}

	enrichConfigurableProduct(context.Background(), nil, prod, "https://fashion-demo.secomm.vn/media")

	colorImages, ok := prod["color_images"].([]map[string]interface{})
	if !ok || len(colorImages) != 3 {
		t.Fatalf("expected 3 items in color_images, got %v", prod["color_images"])
	}

	// Verify Xanh olive has 3 images
	var xanhOliveItem map[string]interface{}
	for _, ci := range colorImages {
		if ci["color"] == "Xanh olive" {
			xanhOliveItem = ci
			break
		}
	}
	if xanhOliveItem == nil {
		t.Fatalf("color 'Xanh olive' not found in color_images")
	}

	gallery, ok := xanhOliveItem["gallery"].([]string)
	if !ok || len(gallery) != 3 {
		t.Errorf("expected 3 gallery images for 'Xanh olive', got %v", xanhOliveItem["gallery"])
	}

	if xanhOliveItem["image_url"] != "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-01_1.jpg" {
		t.Errorf("expected first image for 'Xanh olive', got %v", xanhOliveItem["image_url"])
	}
	if xanhOliveItem["color_code"] != "#808000" {
		t.Errorf("expected #808000 for 'Xanh olive', got %v", xanhOliveItem["color_code"])
	}

	// Verify other colors have empty galleries
	for _, ci := range colorImages {
		if ci["color"] != "Xanh olive" {
			gal := ci["gallery"].([]string)
			if len(gal) != 0 {
				t.Errorf("expected 0 gallery images for %s, got %d", ci["color"], len(gal))
			}
			if ci["image_url"] != "" {
				t.Errorf("expected empty image_url for %s, got %v", ci["color"], ci["image_url"])
			}
		}
	}
}

func TestEnrichConfigurableProduct_WithParentLabels(t *testing.T) {
	prod := map[string]interface{}{
		"id":      float64(100),
		"sku":     "TSHIRT-CONFIG",
		"name":    "Basic T-Shirt",
		"type_id": "configurable",
		"media_gallery_entries": []interface{}{
			map[string]interface{}{
				"file":  "/t/s/tshirt-red.jpg",
				"url":   "https://example.com/media/catalog/product/t/s/tshirt-red.jpg",
				"label": "Đỏ / Red Front",
			},
			map[string]interface{}{
				"file":  "/t/s/tshirt-blue.jpg",
				"url":   "https://example.com/media/catalog/product/t/s/tshirt-blue.jpg",
				"label": "Xanh dương / Blue",
			},
		},
		"extension_attributes": map[string]interface{}{
			"configurable_product_options": []interface{}{
				map[string]interface{}{
					"attribute_id": "93",
					"label":        "Màu sắc",
					"values": []interface{}{
						map[string]interface{}{
							"label":       "Đỏ",
							"value_index": float64(10),
						},
						map[string]interface{}{
							"label":       "Xanh dương",
							"value_index": float64(20),
						},
					},
				},
			},
		},
	}

	enrichConfigurableProduct(context.Background(), nil, prod, "https://example.com/media")

	colorImages, ok := prod["color_images"].([]map[string]interface{})
	if !ok || len(colorImages) != 2 {
		t.Fatalf("expected 2 items in color_images, got %v", prod["color_images"])
	}

	if colorImages[0]["color"] == "Đỏ" {
		if colorImages[0]["image_url"] != "https://example.com/media/catalog/product/t/s/tshirt-red.jpg" {
			t.Errorf("expected red image, got %v", colorImages[0]["image_url"])
		}
	}
	if colorImages[1]["color"] == "Xanh dương" {
		if colorImages[1]["image_url"] != "https://example.com/media/catalog/product/t/s/tshirt-blue.jpg" {
			t.Errorf("expected blue image, got %v", colorImages[1]["image_url"])
		}
	}
}

func TestRunGetProduct_Cleo(t *testing.T) {
	t.Setenv("MAGENTO_TOOLS", "")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/V1/products":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"items": [
					{
						"id": 1636,
						"sku": "WF-SK-013",
						"name": "Chân váy midi phối đai Cleo",
						"type_id": "configurable",
						"visibility": 4,
						"price": 699000
					}
				],
				"total_count": 1
			}`))
		case "/rest/V1/products/WF-SK-013":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": 1636,
				"sku": "WF-SK-013",
				"name": "Chân váy midi phối đai Cleo",
				"type_id": "configurable",
				"visibility": 4,
				"price": 699000,
				"custom_attributes": [
					{
						"attribute_code": "image",
						"value": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-01_1.jpg"
					},
					{
						"attribute_code": "url_key",
						"value": "chan-vay-midi-phoi-dai-cleo-xanh-olive-wf-sk-013"
					},
					{
						"attribute_code": "short_description",
						"value": "<p>Chân váy midi phối đai Cleo – xanh olive. Dáng straight midi; đai cùng vải và xẻ sau. Chất liệu twill cotton pha elastane.</p>"
					},
					{
						"attribute_code": "description",
						"value": "<p>Chân váy midi phối đai Cleo mang sắc xanh olive, phù hợp cho phong cách công sở chỉn chu và những ngày cần trang phục dễ phối.</p><ul><li><strong>Chất liệu:</strong> Twill cotton pha elastane.</li><li><strong>Màu sắc:</strong> Xanh olive.</li></ul>"
					}
				],
				"extension_attributes": {
					"configurable_product_links": [1261, 1262, 1263, 1264, 1265, 1266, 1267, 1268, 1269, 1270, 1271, 1272],
					"configurable_product_options": [
						{
							"attribute_id": "93",
							"id": 244,
							"label": "Màu sắc",
							"position": 0,
							"product_id": 1636,
							"values": [
								{"value_index": 126},
								{"value_index": 136},
								{"value_index": 138}
							]
						},
						{
							"attribute_id": "175",
							"id": 245,
							"label": "Kích cỡ",
							"position": 1,
							"product_id": 1636,
							"values": [
								{"value_index": 97},
								{"value_index": 98},
								{"value_index": 99},
								{"value_index": 100}
							]
						}
					]
				},
				"media_gallery_entries": [
					{
						"disabled": false,
						"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-01_1.jpg",
						"id": 3631,
						"label": "",
						"position": 1,
						"types": ["image", "small_image", "thumbnail"]
					},
					{
						"disabled": false,
						"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-02_1.jpg",
						"id": 3632,
						"label": "",
						"position": 2,
						"types": []
					},
					{
						"disabled": false,
						"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-03_1.jpg",
						"id": 3633,
						"label": "",
						"position": 3,
						"types": []
					}
				]
			}`))
		case "/rest/V1/configurable-products/WF-SK-013/children":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{
					"id": 1261,
					"sku": "WF-SK-013-OLV-S",
					"price": 699000,
					"custom_attributes": [
						{"attribute_code": "color", "value": "138"},
						{"attribute_code": "size", "value": "97"},
						{"attribute_code": "image", "value": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-01_1.jpg"}
					],
					"media_gallery_entries": [
						{"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-01_1.jpg", "position": 1, "disabled": false},
						{"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-02_1.jpg", "position": 2, "disabled": false},
						{"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-03_1.jpg", "position": 3, "disabled": false}
					]
				},
				{
					"id": 1265,
					"sku": "WF-SK-013-DBL-S",
					"price": 699000,
					"custom_attributes": [
						{"attribute_code": "color", "value": "136"},
						{"attribute_code": "size", "value": "97"},
						{"attribute_code": "image", "value": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-dbl-01.jpg"}
					],
					"media_gallery_entries": [
						{"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-dbl-01.jpg", "position": 1, "disabled": false},
						{"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-dbl-02.jpg", "position": 2, "disabled": false},
						{"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-dbl-03.jpg", "position": 3, "disabled": false}
					]
				},
				{
					"id": 1269,
					"sku": "WF-SK-013-LAT-S",
					"price": 699000,
					"custom_attributes": [
						{"attribute_code": "color", "value": "126"},
						{"attribute_code": "size", "value": "97"},
						{"attribute_code": "image", "value": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-lat-01.jpg"}
					],
					"media_gallery_entries": [
						{"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-lat-01.jpg", "position": 1, "disabled": false},
						{"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-lat-02.jpg", "position": 2, "disabled": false},
						{"file": "https://fashion-demo.secomm.vn/media/catalog/product/w/f/wf-sk-013-lat-03.jpg", "position": 3, "disabled": false}
					]
				}
			]`))
		case "/rest/V1/products/attributes/93/options":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"value": "126", "label": "Nâu latte"},
				{"value": "136", "label": "Xanh denim bụi"},
				{"value": "138", "label": "Xanh olive"}
			]`))
		case "/rest/V1/products/attributes/175/options":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"value": "97", "label": "S"},
				{"value": "98", "label": "M"},
				{"value": "99", "label": "L"},
				{"value": "100", "label": "XL"}
			]`))
		case "/graphql":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"data": {
					"products": {
						"items": [{
							"price_range": {
								"minimum_price": {
									"regular_price": {"value": 699000, "currency": "VND"},
									"final_price": {"value": 629100, "currency": "VND"},
									"discount": {"amount_off": 69900, "percent_off": 10}
								},
								"maximum_price": {
									"regular_price": {"value": 699000, "currency": "VND"},
									"final_price": {"value": 629100, "currency": "VND"},
									"discount": {"amount_off": 69900, "percent_off": 10}
								}
							},
							"configurable_options": [{
								"attribute_code": "color",
								"values": [
									{"value_index": 126, "label": "Nâu latte", "swatch_data": {"value": "#b58f72"}},
									{"value_index": 136, "label": "Xanh denim bụi", "swatch_data": {"value": "#6d8295"}},
									{"value_index": 138, "label": "Xanh olive", "swatch_data": {"value": "#808000"}}
								]
							}]
						}]
					}
				}
			}`))
		default:
			t.Errorf("unexpected endpoint requested: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	_, handlers := Register()
	args := map[string]interface{}{
		"name":             "Chân váy midi phối đai Cleo",
		"credentials_json": fmt.Sprintf(`{"base_url":"%s","consumer_key":"ck","consumer_secret":"cs","access_token":"at","access_token_secret":"ats"}`, srv.URL),
	}
	raw, _ := json.Marshal(args)
	res := handlers["magento_get_product"](raw)
	if res["isError"] == true {
		t.Fatalf("unexpected error result: %+v", res)
	}

	content := res["content"].([]map[string]string)[0]["text"]
	t.Logf("OUTPUT_JSON_RESULT:\n%s", content)
}

func TestHandleListProducts_EnrichesPromotionalPrice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/V1/products":
			// Return product with regular price 1,450,000 (such as WF-SH-002)
			_, _ = w.Write([]byte(`{
				"items": [
					{
						"id": 101,
						"sku": "WF-SH-002",
						"name": "Áo sơ mi lụa tơ tằm cổ Đức",
						"price": 1450000,
						"type_id": "configurable"
					}
				],
				"total_count": 1
			}`))
		case "/graphql":
			// GraphQL returns dynamic sale price 1,160,000 with 20% discount
			_, _ = w.Write([]byte(`{
				"data": {
					"products": {
						"items": [
							{
								"sku": "WF-SH-002",
								"price_range": {
									"minimum_price": {
										"regular_price": {
											"value": 1450000,
											"currency": "VND"
										},
										"final_price": {
											"value": 1160000,
											"currency": "VND"
										},
										"discount": {
											"amount_off": 290000,
											"percent_off": 20
										}
									}
								}
							}
						]
					}
				}
			}`))
		default:
			t.Errorf("unexpected endpoint: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	_, handlers := Register()
	args := map[string]interface{}{
		"name":             "áo sơ mi",
		"credentials_json": fmt.Sprintf(`{"base_url":"%s","consumer_key":"ck","consumer_secret":"cs","access_token":"at","access_token_secret":"ats"}`, srv.URL),
	}
	raw, _ := json.Marshal(args)
	res := handlers["magento_list_products"](raw)
	if res["isError"] == true {
		t.Fatalf("unexpected error: %+v", res)
	}

	content := res["content"].([]map[string]string)[0]["text"]
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("failed to parse output json: %v", err)
	}

	items, ok := parsed["items"].([]interface{})
	if !ok || len(items) == 0 {
		t.Fatalf("expected at least 1 item in response")
	}

	item := items[0].(map[string]interface{})
	if item["sku"] != "WF-SH-002" {
		t.Errorf("expected sku WF-SH-002, got %v", item["sku"])
	}
	// Verify discounted price
	if toFloat(item["price"]) != 1160000 {
		t.Errorf("expected price 1160000, got %v", item["price"])
	}
	if toFloat(item["original_price"]) != 1450000 {
		t.Errorf("expected original_price 1450000, got %v", item["original_price"])
	}
	if toFloat(item["special_price"]) != 1160000 {
		t.Errorf("expected special_price 1160000, got %v", item["special_price"])
	}
	if item["discount"] != "-20%" {
		t.Errorf("expected discount -20%%, got %v", item["discount"])
	}
}

