package tools

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"
)

func TestFormatProductMediaURL(t *testing.T) {
	tests := []struct {
		name      string
		mediaBase string
		file      string
		want      string
	}{
		{
			name:      "standard relative file path",
			mediaBase: "https://example.com/media",
			file:      "/m/b/mb04-black-0.jpg",
			want:      "https://example.com/media/catalog/product/m/b/mb04-black-0.jpg",
		},
		{
			name:      "relative file without leading slash",
			mediaBase: "https://example.com/media/",
			file:      "m/b/mb04-black-0.jpg",
			want:      "https://example.com/media/catalog/product/m/b/mb04-black-0.jpg",
		},
		{
			name:      "already full URL",
			mediaBase: "https://example.com/media",
			file:      "https://cdn.example.com/images/foo.jpg",
			want:      "https://cdn.example.com/images/foo.jpg",
		},
		{
			name:      "already starts with catalog/product",
			mediaBase: "https://example.com/media",
			file:      "/catalog/product/m/b/test.png",
			want:      "https://example.com/media/catalog/product/m/b/test.png",
		},
		{
			name:      "empty file",
			mediaBase: "https://example.com/media",
			file:      "",
			want:      "",
		},
		{
			name:      "empty media base",
			mediaBase: "",
			file:      "/m/b/test.png",
			want:      "/m/b/test.png",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatProductMediaURL(tt.mediaBase, tt.file)
			if got != tt.want {
				t.Errorf("formatProductMediaURL(%q, %q) = %q, want %q", tt.mediaBase, tt.file, got, tt.want)
			}
		})
	}
}

func TestEnrichMediaEntry(t *testing.T) {
	entry := map[string]interface{}{
		"id":         float64(2),
		"media_type": "image",
		"label":      "Black Backpack",
		"position":   float64(1),
		"disabled":   false,
		"file":       "/m/b/mb04-black-0.jpg",
		"types":      []interface{}{"image", "small_image", "thumbnail"},
	}

	mediaBase := "https://mage-demo.secomm.vn/media"
	enrichMediaEntry(entry, mediaBase)

	expectedURL := "https://mage-demo.secomm.vn/media/catalog/product/m/b/mb04-black-0.jpg"

	if entry["url"] != expectedURL {
		t.Errorf("expected url to be %q, got %q", expectedURL, entry["url"])
	}
	if entry["full_url"] != expectedURL {
		t.Errorf("expected full_url to be %q, got %q", expectedURL, entry["full_url"])
	}
	if entry["file"] != expectedURL {
		t.Errorf("expected file to be replaced with full URL %q, got %q", expectedURL, entry["file"])
	}
	if entry["file_path"] != "/m/b/mb04-black-0.jpg" {
		t.Errorf("expected file_path to preserve original relative path, got %q", entry["file_path"])
	}
}

func TestEnrichProductMedia(t *testing.T) {
	prod := map[string]interface{}{
		"id":   float64(1),
		"sku":  "24-MB04",
		"name": "Strive Shoulder Pack",
		"media_gallery_entries": []interface{}{
			map[string]interface{}{
				"id":   float64(2),
				"file": "/m/b/mb04-black-0.jpg",
			},
		},
		"custom_attributes": []interface{}{
			map[string]interface{}{
				"attribute_code": "image",
				"value":          "/m/b/mb04-black-0.jpg",
			},
			map[string]interface{}{
				"attribute_code": "thumbnail",
				"value":          "/m/b/mb04-black-0.jpg",
			},
		},
	}

	mediaBase := "https://mage-demo.secomm.vn/media"
	enrichProductMedia(prod, mediaBase)

	expectedURL := "https://mage-demo.secomm.vn/media/catalog/product/m/b/mb04-black-0.jpg"
	if prod["image_url"] != expectedURL {
		t.Errorf("expected prod image_url %q, got %q", expectedURL, prod["image_url"])
	}

	entries := prod["media_gallery_entries"].([]interface{})
	firstEntry := entries[0].(map[string]interface{})
	if firstEntry["url"] != expectedURL {
		t.Errorf("expected entry url %q, got %q", expectedURL, firstEntry["url"])
	}

	cas := prod["custom_attributes"].([]interface{})
	firstCA := cas[0].(map[string]interface{})
	if firstCA["value"] != expectedURL {
		t.Errorf("expected ca value %q, got %q", expectedURL, firstCA["value"])
	}
}

func TestHandleMagentoListProductMedia_Mock(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/V1/store/storeConfigs":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"base_media_url":        "http://" + r.Host + "/media/",
					"secure_base_media_url": "http://" + r.Host + "/media/",
				},
			})
		case "/rest/V1/products/24-MB04/media":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"id":         1,
					"media_type": "image",
					"label":      "Product 1",
					"file":       "/m/b/mb04-black-0.jpg",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	credsJSON, _ := json.Marshal(map[string]interface{}{
		"base_url":     mockServer.URL,
		"access_token": "test-token",
		"auth_mode":    "bearer",
	})

	args, _ := json.Marshal(map[string]interface{}{
		"credentials_json": string(credsJSON),
		"sku":              "24-MB04",
	})

	res := handleMagentoListProductMedia(args)
	if isErr, _ := res["isError"].(bool); isErr {
		t.Fatalf("unexpected error result: %v", res)
	}

	contentArr, ok := res["content"].([]map[string]string)
	if !ok || len(contentArr) == 0 {
		t.Fatalf("missing text content: %v", res)
	}
	text := contentArr[0]["text"]

	expectedURL := mockServer.URL + "/media/catalog/product/m/b/mb04-black-0.jpg"
	if !strings.Contains(text, expectedURL) {
		t.Fatalf("expected output to contain complete URL %q, got:\n%s", expectedURL, text)
	}
	if !strings.Contains(text, `"url":`) {
		t.Fatalf("expected output to contain 'url' field, got:\n%s", text)
	}
	if !strings.Contains(text, `"file_path": "/m/b/mb04-black-0.jpg"`) {
		t.Fatalf("expected output to contain 'file_path' field, got:\n%s", text)
	}
}

func TestHandleMagentoGetProductMedia_Mock(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/V1/store/storeConfigs":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{
					"base_media_url": "http://" + r.Host + "/media/",
				},
			})
		case "/rest/V1/products/24-MB04/media/1":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id":         1,
				"media_type": "image",
				"label":      "Product 1",
				"file":       "/m/b/mb04-black-0.jpg",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	credsJSON, _ := json.Marshal(map[string]interface{}{
		"base_url":     mockServer.URL,
		"access_token": "test-token",
		"auth_mode":    "bearer",
	})

	args, _ := json.Marshal(map[string]interface{}{
		"credentials_json": string(credsJSON),
		"sku":              "24-MB04",
		"entry_id":         "1",
	})

	res := handleMagentoGetProductMedia(args)
	if isErr, _ := res["isError"].(bool); isErr {
		t.Fatalf("unexpected error result: %v", res)
	}

	contentArr, ok := res["content"].([]map[string]string)
	if !ok || len(contentArr) == 0 {
		t.Fatalf("missing text content: %v", res)
	}
	text := contentArr[0]["text"]

	expectedURL := mockServer.URL + "/media/catalog/product/m/b/mb04-black-0.jpg"
	if !strings.Contains(text, expectedURL) {
		t.Fatalf("expected output to contain complete URL %q, got:\n%s", expectedURL, text)
	}
	if !strings.Contains(text, `"url":`) {
		t.Fatalf("expected output to contain 'url' field, got:\n%s", text)
	}
}

func TestLive_MagentoListProductMedia(t *testing.T) {
	_ = godotenv.Load("../../.env")
	if os.Getenv("MAGENTO_BASE_URL") == "" {
		t.Skip("skipping live test: MAGENTO_BASE_URL not set")
	}

	args, _ := json.Marshal(map[string]interface{}{
		"sku": "24-MB04",
	})

	res := handleMagentoListProductMedia(args)
	if isErr, _ := res["isError"].(bool); isErr {
		t.Fatalf("live media query failed: %v", res)
	}

	contentArr, ok := res["content"].([]map[string]string)
	if !ok || len(contentArr) == 0 {
		t.Fatalf("missing text content: %v", res)
	}
	text := contentArr[0]["text"]
	t.Logf("LIVE RESPONSE:\n%s", text)

	expectedBase := os.Getenv("MAGENTO_BASE_URL") + "/media/catalog/product"
	if !strings.Contains(text, expectedBase) {
		t.Fatalf("expected output to contain %q, got:\n%s", expectedBase, text)
	}
}

