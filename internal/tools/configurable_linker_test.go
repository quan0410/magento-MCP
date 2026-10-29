package tools

import (
	"bytes"
	"image"
	"image/jpeg"
	"reflect"
	"testing"
)

func TestParseConfigurableVariations(t *testing.T) {
	raw := "sku=TSHIRT-RED-S,size=S,color=Red|sku=TSHIRT-RED-M,size=M,color=Red"
	items := parseConfigurableVariations(raw)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].ChildSKU != "TSHIRT-RED-S" {
		t.Errorf("expected child sku TSHIRT-RED-S, got %s", items[0].ChildSKU)
	}
	if items[0].Attributes["size"] != "S" || items[0].Attributes["color"] != "Red" {
		t.Errorf("unexpected attributes for item 0: %v", items[0].Attributes)
	}

	if items[1].ChildSKU != "TSHIRT-RED-M" {
		t.Errorf("expected child sku TSHIRT-RED-M, got %s", items[1].ChildSKU)
	}
}

func TestGroupConfigurableProducts(t *testing.T) {
	rows := []ParsedProductRow{
		{
			RowNumber: 2,
			SKU:       "TSHIRT-RED-PARENT",
			Name:      "T-Shirt Red",
			TypeID:    "configurable",
			ConfigurableVariations: "sku=TSHIRT-RED-S,size=S",
		},
		{
			RowNumber: 3,
			SKU:       "TSHIRT-RED-S",
			Name:      "T-Shirt Red S",
			TypeID:    "simple",
			CustomAttributes: map[string]interface{}{"size": "S"},
		},
		{
			RowNumber: 4,
			SKU:       "TSHIRT-RED-M",
			Name:      "T-Shirt Red M",
			TypeID:    "simple",
			CustomAttributes: map[string]interface{}{"size": "M"},
		},
		{
			RowNumber: 5,
			SKU:       "OTHER-SIMPLE",
			Name:      "Other",
			TypeID:    "simple",
		},
	}

	simples, groups := groupConfigurableProducts(rows)

	if len(simples) != 3 {
		t.Errorf("expected 3 simple rows, got %d", len(simples))
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 configurable group, got %d", len(groups))
	}

	group := groups[0]
	if group.ParentRow.SKU != "TSHIRT-RED-PARENT" {
		t.Errorf("expected parent SKU TSHIRT-RED-PARENT, got %s", group.ParentRow.SKU)
	}

	// Should have matched both TSHIRT-RED-S (from variations/prefix) and TSHIRT-RED-M (from prefix)
	expectedSKUs := []string{"TSHIRT-RED-S", "TSHIRT-RED-M"}
	var actualSKUs []string
	for _, c := range group.Children {
		actualSKUs = append(actualSKUs, c.SKU)
	}

	if !reflect.DeepEqual(actualSKUs, expectedSKUs) {
		t.Errorf("expected children %v, got %v", expectedSKUs, actualSKUs)
	}
}

func TestParseConfigurableVariationLabels(t *testing.T) {
	raw := "size=Size, color=Color, brand=Brand Name"
	labels := parseConfigurableVariationLabels(raw)

	if labels["size"] != "Size" {
		t.Errorf("expected Size, got %s", labels["size"])
	}
	if labels["color"] != "Color" {
		t.Errorf("expected Color, got %s", labels["color"])
	}
	if labels["brand"] != "Brand Name" {
		t.Errorf("expected Brand Name, got %s", labels["brand"])
	}
}

func TestGroupConfigurableProducts_NoParentSuffix(t *testing.T) {
	// Standard Magento export where parent SKU is "TSHIRT-COTTON-RED" and children are "TSHIRT-COTTON-RED-S", etc.
	rows := []ParsedProductRow{
		{
			RowNumber: 2,
			SKU:       "TSHIRT-COTTON-RED",
			Name:      "Men Cotton T-Shirt Red",
			TypeID:    "configurable",
			ConfigurableVariationLabels: "size=Size",
		},
		{
			RowNumber: 3,
			SKU:       "TSHIRT-COTTON-RED-S",
			Name:      "Men Cotton T-Shirt Red - S",
			TypeID:    "simple",
			CustomAttributes: map[string]interface{}{"size": "S"},
		},
		{
			RowNumber: 4,
			SKU:       "TSHIRT-COTTON-RED-M",
			Name:      "Men Cotton T-Shirt Red - M",
			TypeID:    "simple",
			CustomAttributes: map[string]interface{}{"size": "M"},
		},
	}

	simples, groups := groupConfigurableProducts(rows)

	if len(simples) != 2 {
		t.Errorf("expected 2 simples, got %d", len(simples))
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Children) != 2 {
		t.Fatalf("expected 2 children linked, got %d", len(groups[0].Children))
	}
	if groups[0].Children[0].SKU != "TSHIRT-COTTON-RED-S" || groups[0].Children[1].SKU != "TSHIRT-COTTON-RED-M" {
		t.Errorf("unexpected children SKUs: %v", groups[0].Children)
	}
}

func TestAlignSmartChunkBoundary(t *testing.T) {
	// 2 configurable groups:
	// Group 1: parent at 0, children at 1, 2, 3 (4 rows total)
	// Group 2: parent at 4, children at 5, 6, 7 (4 rows total)
	rows := []ParsedProductRow{
		{RowNumber: 2, SKU: "PARENT-1", TypeID: "configurable", ConfigurableVariations: "sku=CHILD-1-A|sku=CHILD-1-B|sku=CHILD-1-C"},
		{RowNumber: 3, SKU: "CHILD-1-A", TypeID: "simple"},
		{RowNumber: 4, SKU: "CHILD-1-B", TypeID: "simple"},
		{RowNumber: 5, SKU: "CHILD-1-C", TypeID: "simple"},
		{RowNumber: 6, SKU: "PARENT-2", TypeID: "configurable", ConfigurableVariations: "sku=CHILD-2-A|sku=CHILD-2-B|sku=CHILD-2-C"},
		{RowNumber: 7, SKU: "CHILD-2-A", TypeID: "simple"},
		{RowNumber: 8, SKU: "CHILD-2-B", TypeID: "simple"},
		{RowNumber: 9, SKU: "CHILD-2-C", TypeID: "simple"},
	}

	// Case 1: Limit = 2 cuts Group 1 in half. Smart Chunking should auto-expand to include all of Group 1 (4 rows)
	active1, next1, hasMore1 := alignSmartChunkBoundary(rows, 0, 2)
	if len(active1) != 4 {
		t.Fatalf("expected active1 to expand to 4 rows, got %d", len(active1))
	}
	if next1 != 4 {
		t.Errorf("expected nextOffset 4, got %d", next1)
	}
	if !hasMore1 {
		t.Errorf("expected hasMore true")
	}

	// Case 2: Second batch starting at offset 4 with limit = 2. Should expand to remaining 4 rows (offset 4 to 8)
	active2, next2, hasMore2 := alignSmartChunkBoundary(rows, 4, 2)
	if len(active2) != 4 {
		t.Fatalf("expected active2 to expand to 4 rows, got %d", len(active2))
	}
	if next2 != 0 || hasMore2 {
		t.Errorf("expected end of dataset, got next=%d, hasMore=%v", next2, hasMore2)
	}
}

func TestDeferredMediaQueueStatus(t *testing.T) {
	q := &deferredMediaQueue{}
	status := q.GetStatus()
	if status.IsRunning {
		t.Errorf("expected queue not running initially")
	}
	if status.TotalQueued != 0 {
		t.Errorf("expected 0 total queued, got %d", status.TotalQueued)
	}
}

func TestResizeImageIfExceedsMaxDim(t *testing.T) {
	// Create a dummy 2000x1500 image in memory
	src := image.NewRGBA(image.Rect(0, 0, 2000, 1500))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("failed to encode test jpeg: %v", err)
	}

	resizedData, mimeType := resizeImageIfExceedsMaxDim(buf.Bytes(), "image/jpeg", 1200)
	if mimeType != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", mimeType)
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(resizedData))
	if err != nil {
		t.Fatalf("failed to decode resized jpeg: %v", err)
	}

	if cfg.Width != 1200 {
		t.Errorf("expected width 1200, got %d", cfg.Width)
	}
	if cfg.Height != 900 {
		t.Errorf("expected height 900, got %d", cfg.Height)
	}
}



