package tools

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// ParsedProductRow represents a unified, strongly-typed product row extracted from CSV or Excel.
type ParsedProductRow struct {
	RowNumber        int
	SKU              string
	Name             string
	Price            *float64
	AttributeSetID   *int
	TypeID           string
	Status           *int
	Visibility       *int
	Weight           *float64
	Qty              *float64
	IsInStock        *int
	ManageStock      *int
	SpecialPrice     *float64
	SpecialFromDate  string
	SpecialToDate    string
	Cost             *float64
	MSRP             *float64
	Categories       string // comma-separated category IDs, e.g. "2,15,34"
	TaxClassID       *int
	ShortDescription string
	Description      string
	URLKey           string
	MetaTitle        string
	MetaKeywords     string
	MetaDescription  string
	ImageURL               string
	AdditionalImages            string
	VideoURL                    string
	VideoTitle                  string
	ConfigurableVariations      string // raw string from configurable_variations column e.g. "sku=CHILD-1,size=S|sku=CHILD-2,size=M"
	ConfigurableVariationLabels string // raw string from configurable_variation_labels column e.g. "size=Size,color=Color"
	CustomAttributes            map[string]interface{}
}

// loadFileBytes reads file content from a local file path.
func loadFileBytes(filePath string) ([]byte, string, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(filePath))
	if cleanPath == "" || cleanPath == "." {
		return nil, "", errors.New("missing required argument 'file_path'")
	}
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read local file '%s': %w", filePath, err)
	}
	ext := strings.ToLower(filepath.Ext(cleanPath))
	return data, ext, nil
}

// parseProductsFile parses Excel (.xlsx) bytes into a slice of ParsedProductRow.
func parseProductsFile(data []byte, ext string) ([]ParsedProductRow, error) {
	isZipXLSX := len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04
	if !strings.HasPrefix(ext, ".xlsx") && !isZipXLSX {
		return nil, errors.New("unsupported file format: only Excel (.xlsx) files are supported")
	}
	return parseExcelFile(data)
}

// parseCSVFile parses CSV data supporting comma or semicolon delimiters.
func parseCSVFile(data []byte) ([]ParsedProductRow, error) {
	// Strip UTF-8 BOM if present
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))

	// Detect delimiter (default comma, check for semicolon)
	delimiter := rune(',')
	if firstLine := bytes.SplitN(data, []byte("\n"), 2); len(firstLine) > 0 {
		if bytes.Count(firstLine[0], []byte(";")) > bytes.Count(firstLine[0], []byte(",")) {
			delimiter = ';'
		}
	}

	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = delimiter
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, errors.New("file is empty or contains only header row")
	}

	header := records[0]
	var rows []ParsedProductRow

	for i := 1; i < len(records); i++ {
		record := records[i]
		// skip completely empty rows
		allEmpty := true
		for _, cell := range record {
			if strings.TrimSpace(cell) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}

		row := mapRecordToProductRow(i+1, header, record)
		rows = append(rows, row)
	}

	return rows, nil
}

// parseExcelFile parses Excel (.xlsx) data using excelize.
func parseExcelFile(data []byte) ([]ParsedProductRow, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("Excel file has no worksheets")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read rows from sheet '%s': %w", sheets[0], err)
	}
	if len(rows) < 2 {
		return nil, errors.New("Excel sheet is empty or contains only header row")
	}

	header := rows[0]
	var result []ParsedProductRow

	for i := 1; i < len(rows); i++ {
		record := rows[i]
		allEmpty := true
		for _, cell := range record {
			if strings.TrimSpace(cell) != "" {
				allEmpty = false
				break
			}
		}
		if allEmpty {
			continue
		}

		row := mapRecordToProductRow(i+1, header, record)
		result = append(result, row)
	}

	return result, nil
}

// normalizeHeaderKey standardizes header column names.
func normalizeHeaderKey(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// mapRecordToProductRow maps a single row into ParsedProductRow with full Magento schema support.
func mapRecordToProductRow(rowNum int, header []string, record []string) ParsedProductRow {
	p := ParsedProductRow{
		RowNumber:        rowNum,
		CustomAttributes: make(map[string]interface{}),
	}

	for idx, colName := range header {
		if idx >= len(record) {
			break
		}
		val := strings.TrimSpace(record[idx])
		if val == "" {
			continue
		}

		normKey := normalizeHeaderKey(colName)

		switch normKey {
		case "sku", "product_sku":
			p.SKU = val
		case "name", "product_name", "title":
			p.Name = val
		case "price", "regular_price", "unit_price":
			p.Price = parseCleanFloat(val)
		case "attribute_set_id", "attribute_set":
			if id, err := strconv.Atoi(val); err == nil {
				p.AttributeSetID = &id
			}
		case "type_id", "type", "product_type":
			p.TypeID = strings.ToLower(val)
		case "status":
			if val == "1" || strings.EqualFold(val, "enabled") || strings.EqualFold(val, "true") {
				one := 1
				p.Status = &one
			} else if val == "2" || strings.EqualFold(val, "disabled") || strings.EqualFold(val, "false") {
				two := 2
				p.Status = &two
			}
		case "visibility":
			if v, err := strconv.Atoi(val); err == nil {
				p.Visibility = &v
			}
		case "weight":
			p.Weight = parseCleanFloat(val)
		case "qty", "quantity", "stock", "stock_qty":
			p.Qty = parseCleanFloat(val)
		case "is_in_stock":
			if val == "1" || strings.EqualFold(val, "true") || strings.EqualFold(val, "yes") {
				one := 1
				p.IsInStock = &one
			} else {
				zero := 0
				p.IsInStock = &zero
			}
		case "manage_stock":
			if val == "1" || strings.EqualFold(val, "true") || strings.EqualFold(val, "yes") {
				one := 1
				p.ManageStock = &one
			} else {
				zero := 0
				p.ManageStock = &zero
			}
		case "special_price":
			p.SpecialPrice = parseCleanFloat(val)
		case "special_from_date", "special_from":
			p.SpecialFromDate = normalizeImportDate(val)
		case "special_to_date", "special_to":
			p.SpecialToDate = normalizeImportDate(val)
		case "cost":
			p.Cost = parseCleanFloat(val)
		case "msrp":
			p.MSRP = parseCleanFloat(val)
		case "categories", "category_ids", "category":
			p.Categories = val
		case "tax_class_id", "tax_class":
			if t, err := strconv.Atoi(val); err == nil {
				p.TaxClassID = &t
			}
		case "short_description":
			p.ShortDescription = val
		case "description":
			p.Description = val
		case "url_key", "slug":
			p.URLKey = val
		case "meta_title":
			p.MetaTitle = val
		case "meta_keywords", "meta_keyword":
			p.MetaKeywords = val
		case "meta_description":
			p.MetaDescription = val
		case "image_url", "image", "base_image":
			p.ImageURL = val
		case "additional_images", "gallery_images", "images":
			p.AdditionalImages = val
		case "video_url", "video":
			p.VideoURL = val
		case "video_title":
			p.VideoTitle = val
		case "configurable_variations", "configurable_variation", "variations", "variation":
			p.ConfigurableVariations = val
		case "configurable_variation_labels", "variation_labels", "configurable_labels":
			p.ConfigurableVariationLabels = val
		default:
			// Dynamic custom attributes (e.g. color, size, brand, manufacturer)
			p.CustomAttributes[normKey] = val
		}
	}

	return p
}

// normalizeImportDate parses various human or Excel date formats into standard YYYY-MM-DD.
func normalizeImportDate(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}

	// 1. If it's an Excel numeric serial date (e.g. 46327 or 46327.0)
	if f, err := strconv.ParseFloat(val, 64); err == nil && f > 20000 && f < 80000 {
		excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		t := excelEpoch.Add(time.Duration(f * float64(24*time.Hour)))
		return t.Format("2006-01-02")
	}

	// 2. Try common date formats
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006/01/02",
		"2006/01/02 15:04:05",
		"01-02-06",          // e.g. "12-31-26" -> Dec 31, 2026
		"01-02-2006",        // e.g. "12-31-2026"
		"01/02/2006",        // e.g. "12/31/2026"
		"01/02/06",          // e.g. "12/31/26"
		"02-01-2006",
		"02/01/2006",
		"January 2, 2006",
		"2 Jan 2006",
		time.RFC3339,
	}

	for _, layout := range formats {
		if t, err := time.Parse(layout, val); err == nil {
			return t.Format("2006-01-02")
		}
	}

	return val
}

// parseCleanFloat sanitizes numeric strings containing currency symbols ($ € £ ¥ ₫),
// commas, or unit text and parses them into a float64 pointer.
func parseCleanFloat(val string) *float64 {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}

	// Remove common currency symbols and labels
	for _, sym := range []string{"$", "€", "£", "¥", "₫", "USD", "VND", "EUR", "GBP", "usd", "vnd"} {
		val = strings.ReplaceAll(val, sym, "")
	}
	val = strings.TrimSpace(val)

	// Clean thousands separators and decimals
	if strings.Contains(val, ",") && strings.Contains(val, ".") {
		val = strings.ReplaceAll(val, ",", "")
	} else if strings.Contains(val, ",") && !strings.Contains(val, ".") {
		val = strings.ReplaceAll(val, ",", ".")
	}

	if f, err := strconv.ParseFloat(val, 64); err == nil {
		return &f
	}
	return nil
}

