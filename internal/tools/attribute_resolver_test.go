package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datumbridge/magento-mcp/internal/magento"
)

func TestAttributeOptionResolver(t *testing.T) {
	// Mock Magento API server returning color options
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/V1/products/attributes/color/options":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[
				{"label":" ","value":""},
				{"label":"Black","value":"49"},
				{"label":"Blue","value":"50"},
				{"label":"Red","value":"58"}
			]`))
		case "/rest/V1/products/attributes/size/options":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if r.Method == http.MethodPost {
				_, _ = w.Write([]byte(`"212"`))
				return
			}
			_, _ = w.Write([]byte(`[
				{"label":"XS","value":"166"},
				{"label":"S","value":"167"},
				{"label":"M","value":"168"},
				{"label":"L","value":"169"}
			]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"The attribute with a code doesn't exist"}`))
		}
	}))
	defer mockServer.Close()

	client := magento.NewClient(&magento.Credentials{
		BaseURL:     mockServer.URL,
		AccessToken: "test-token",
		AuthMode:    "bearer",
	})

	resolver := newAttributeOptionResolver(client)
	ctx := context.Background()

	// 1. Direct label mapping (Red -> 58 as int)
	resolvedColor := resolver.Resolve(ctx, "color", "Red")
	if resolvedColor != 58 {
		t.Fatalf("expected 58 for Red, got %v (%T)", resolvedColor, resolvedColor)
	}

	// 2. Case-insensitive and trimmed (  rEd   -> 58)
	resolvedColorCase := resolver.Resolve(ctx, "color", "  rEd  ")
	if resolvedColorCase != 58 {
		t.Fatalf("expected 58 for '  rEd  ', got %v", resolvedColorCase)
	}

	// 3. Passing numeric ID directly (58 or "58" -> 58)
	resolvedExistingID := resolver.Resolve(ctx, "color", 58)
	if resolvedExistingID != 58 {
		t.Fatalf("expected 58 for existing ID 58, got %v", resolvedExistingID)
	}

	// 4. Size attribute resolution (M -> 168)
	resolvedSize := resolver.Resolve(ctx, "size", "M")
	if resolvedSize != 168 {
		t.Fatalf("expected 168 for size M, got %v", resolvedSize)
	}

	// 5. Multi-select comma-separated options ("Red, Blue" -> "58,50")
	resolvedMulti := resolver.Resolve(ctx, "color", "Red, Blue")
	if resolvedMulti != "58,50" {
		t.Fatalf("expected '58,50' for 'Red, Blue', got %v", resolvedMulti)
	}

	// 6. Unknown / non-select attribute ("brand" -> "DatumBrand")
	resolvedBrand := resolver.Resolve(ctx, "brand", "DatumBrand")
	if resolvedBrand != "DatumBrand" {
		t.Fatalf("expected 'DatumBrand' unchanged, got %v", resolvedBrand)
	}

	// 7. Auto-create missing option on the fly ("One Size" -> 212)
	resolvedOneSize := resolver.Resolve(ctx, "size", "One Size")
	if resolvedOneSize != 212 {
		t.Fatalf("expected 212 for newly created option 'One Size', got %v", resolvedOneSize)
	}
}
