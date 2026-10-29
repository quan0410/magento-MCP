package tools

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/datumbridge/magento-mcp/internal/mcp"
	"github.com/xuri/excelize/v2"
)

func registerProductImportTemplateTools(add toolAdder) {
	add(
		"magento_get_product_import_template",
		"Generate standard Excel (.xlsx) import template file with sample data and column definitions for creating or updating products in Magento 2.",
		baseProps(map[string]interface{}{
			"type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"create", "update"},
				"default":     "create",
				"description": "Template type: 'create' for adding new products, or 'update' for modifying existing products.",
			},
		}),
		nil,
		handleMagentoGetProductImportTemplate,
	)
}

func handleMagentoGetProductImportTemplate(args json.RawMessage) map[string]interface{} {
	var m map[string]interface{}
	if err := json.Unmarshal(args, &m); err != nil {
		m = map[string]interface{}{}
	}
	templateType := strings.ToLower(strArg(m, "type"))
	return GenerateProductImportTemplate(templateType)
}

// GenerateProductImportTemplate generates standard import templates for Magento products in Excel (.xlsx) format.
func GenerateProductImportTemplate(templateType string) map[string]interface{} {
	templateType = strings.ToLower(templateType)
	if templateType != "update" {
		templateType = "create"
	}

	var headers []string
	var sampleRows [][]string

	if templateType == "create" {
		headers = []string{
			"sku", "name", "price", "cost", "special_price", "special_from_date", "special_to_date",
			"qty", "is_in_stock", "manage_stock", "status", "visibility", "type_id", "attribute_set_id",
			"weight", "categories", "tax_class_id", "short_description", "description",
			"url_key", "meta_title", "meta_keywords", "meta_description",
			"image_url", "additional_images", "video_url", "video_title",
			"brand", "color", "size", "material", "country_of_manufacture",
		}
		sampleRows = [][]string{
			{
				"TSHIRT-RED-M", "Men Cotton T-Shirt Red M", "250000", "150000", "199000", "2026-11-01", "2026-11-30",
				"100", "1", "1", "1", "4", "simple", "4",
				"0.25", "2,15", "2", "Premium soft cotton t-shirt for men", "<p>Detailed description: 100% breathable organic cotton.</p>",
				"men-cotton-tshirt-red-m", "Men Cotton T-Shirt Red M - MyStore", "cotton tshirt, men tshirt red", "Buy Men Cotton T-Shirt Red size M at best price.",
				"https://images.unsplash.com/photo-1521572267360-ee0c2909d518", "https://images.unsplash.com/photo-1503342217505-b0a15ec3261c,https://images.unsplash.com/photo-1583743814966-8936f5b7be1a", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "T-Shirt Product Showcase",
				"DatumBrand", "Red", "M", "100% Organic Cotton", "VN",
			},
			{
				"JEANS-BLUE-32", "Slim Fit Denim Jeans 32", "450000", "280000", "", "", "",
				"50", "1", "1", "1", "4", "simple", "4",
				"0.60", "2,18", "2", "Classic blue slim fit jeans", "<p>Durable denim with slight stretch for daily comfort.</p>",
				"slim-fit-denim-jeans-32", "Slim Fit Denim Jeans 32 - MyStore", "denim jeans, blue slim fit", "Comfortable blue denim jeans size 32.",
				"https://images.unsplash.com/photo-1542272604-780c96856592", "https://images.unsplash.com/photo-1541099649105-f69ad21f3246", "", "",
				"DatumBrand", "Blue", "32", "98% Denim Cotton 2% Spandex", "VN",
			},
		}
	} else {
		// Update template: comprehensive sku with modifiable attributes
		headers = []string{
			"sku", "name", "price", "cost", "special_price", "special_from_date", "special_to_date",
			"qty", "is_in_stock", "status", "visibility", "weight", "categories",
			"short_description", "description", "url_key", "meta_title", "meta_description",
			"image_url", "additional_images", "video_url", "video_title",
			"brand", "color", "size",
		}
		sampleRows = [][]string{
			{
				"TSHIRT-RED-M", "Men Cotton T-Shirt Red M (Updated)", "270000", "160000", "220000", "2026-12-01", "2026-12-31",
				"150", "1", "1", "4", "0.25", "2,15",
				"Updated short description", "<p>Updated detailed description for holiday season.</p>", "men-cotton-tshirt-red-m", "Men Cotton T-Shirt Red M - Holiday Sale", "Updated meta description.",
				"https://images.unsplash.com/photo-1521572267360-ee0c2909d518", "https://images.unsplash.com/photo-1503342217505-b0a15ec3261c", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "Updated Product Video",
				"DatumBrand", "Red", "M",
			},
			{
				"JEANS-BLUE-32", "", "480000", "", "", "", "",
				"80", "1", "1", "", "", "",
				"", "", "", "", "",
				"", "", "", "",
				"", "", "",
			},
		}
	}

	fieldsGuide := map[string]interface{}{
		"sku":                     "Product SKU (Required). Unique identifier in Magento.",
		"name":                    "Product Display Name (Required for create).",
		"price":                   "Product Regular Unit Price (Required for create). Number format.",
		"cost":                    "Cost price of product for merchant profit calculation. Optional.",
		"special_price":           "Promotional discounted price. Optional.",
		"special_from_date":       "Promo start date (YYYY-MM-DD). Optional.",
		"special_to_date":         "Promo end date (YYYY-MM-DD). Optional.",
		"qty":                     "Inventory stock quantity (e.g. 100). Optional.",
		"is_in_stock":             "Stock status: 1 = In Stock, 0 = Out of Stock. Optional (default 1 if qty > 0).",
		"manage_stock":           "1 = Manage stock, 0 = Do not manage stock. Optional.",
		"status":                  "Product status: 1 = Enabled, 2 = Disabled. Optional (default 1).",
		"visibility":              "1 = Not Visible, 2 = Catalog, 3 = Search, 4 = Catalog & Search (default 4).",
		"type_id":                 "Product type: simple, virtual, downloadable (default simple).",
		"attribute_set_id":        "Attribute set ID (default 4 for Default). Optional.",
		"weight":                  "Weight for shipping calculation in kg. Optional.",
		"categories":              "Comma-separated category IDs (e.g. '2,15,34'). Optional.",
		"tax_class_id":            "Tax class ID (e.g. 0 for None, 2 for Taxable Goods). Optional.",
		"short_description":       "Short summary text. Optional.",
		"description":             "Full rich product description (HTML supported). Optional.",
		"url_key":                 "SEO friendly URL slug. Optional.",
		"meta_title":              "SEO title tag. Optional.",
		"meta_keywords":           "SEO meta keywords tag. Optional.",
		"meta_description":        "SEO meta description tag. Optional.",
		"image_url":               "Main/Base image direct URL (automatically downloaded and set to base, small, thumbnail). Optional.",
		"additional_images":       "Comma-separated extra gallery image direct URLs. Multiple images supported. Optional.",
		"video_url":               "External video URL (YouTube or Vimeo direct link) embedded into Images and Videos gallery. Optional.",
		"video_title":             "Title/label for the embedded video. Optional.",
		"brand":                   "Brand name. Mapped to custom attribute.",
		"color":                   "Color attribute (e.g. Red, Blue). Mapped to custom attribute.",
		"size":                    "Size attribute (e.g. M, L, 32). Mapped to custom attribute.",
		"material":                "Material composition (e.g. 100% Cotton). Mapped to custom attribute.",
		"country_of_manufacture":  "Country of manufacture ISO code (e.g. VN, US). Mapped to custom attribute.",
		"custom_attributes":       "Any additional columns are dynamically mapped to Magento custom attributes.",
	}

	filename := fmt.Sprintf("magento_product_%s_template.xlsx", templateType)

	// XLSX format
	f := excelize.NewFile()
	sheet := "Products"
	f.SetSheetName("Sheet1", sheet)

	// Write header
	for colIdx, colName := range headers {
		cell, _ := excelize.CoordinatesToCellName(colIdx+1, 1)
		_ = f.SetCellValue(sheet, cell, colName)
	}

	// Write samples
	for rowIdx, row := range sampleRows {
		for colIdx, val := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			_ = f.SetCellValue(sheet, cell, val)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return mcp.ToolResultError(fmt.Sprintf("failed to generate Excel workbook: %v", err))
	}

	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	return map[string]interface{}{
		"success":      true,
		"type":         templateType,
		"format":       "xlsx",
		"filename":     filename,
		"file_base64":  b64,
		"fields_guide": fieldsGuide,
	}
}
