package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datumbridge/magento-mcp/internal/magento"
	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerProductImportTools(add toolAdder) {
	add(
		"magento_import_create_products",
		"Bulk import and create new products in Magento 2 from an Excel (.xlsx) file. Automatically downloads and uploads product images from URLs. Supports batching (offset/limit), smart chunking, deferred media queue, and high-concurrency for large files. Required columns: sku, name, price.",
		baseProps(map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "Local file path to the Excel (.xlsx) file on disk or container.",
			},
			"skip_existing": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, products whose SKU already exists in Magento will be skipped with a warning instead of failing the row. Useful when retrying partial imports. Default false.",
			},
			"skip_images": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, completely skips downloading and uploading product images and videos. Greatly speeds up product creation. Default false.",
			},
			"defer_images": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, products and configurable links are created immediately while images are queued and uploaded asynchronously in the background. Prevents client timeouts. Check progress via 'magento_get_media_import_status'. Default false.",
			},
			"smart_chunking": map[string]interface{}{
				"type":        "boolean",
				"description": "If true, automatically adjusts the batch boundary so that no Configurable Parent-Child group is split in the middle across batches. Default true.",
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Zero-based row offset to start importing from (useful for chunked/batch imports of large files). Default 0.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of rows to process in this batch (e.g. 25 or 50). If 0 or omitted, processes all remaining rows. Prevents timeouts on large files.",
			},
		}),
		[]string{"file_path"},
		handleMagentoImportCreateProducts,
	)

	add(
		"magento_import_update_products",
		"Bulk update existing products in Magento 2 from an Excel (.xlsx) file. Supports partial updates, batching (offset/limit), and high-concurrency. Required column: sku.",
		baseProps(map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "Local file path to the Excel (.xlsx) file on disk or container.",
			},
			"offset": map[string]interface{}{
				"type":        "integer",
				"description": "Zero-based row offset to start updating from (useful for chunked/batch updates of large files). Default 0.",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of rows to process in this batch (e.g. 50 or 100). If 0 or omitted, processes all remaining rows. Prevents timeouts on large files.",
			},
		}),
		[]string{"file_path"},
		handleMagentoImportUpdateProducts,
	)

	add(
		"magento_get_media_import_status",
		"Check the current progress and status of background (deferred) product image and video uploads in Magento. Returns queued, completed, failed, and pending task counts.",
		baseProps(map[string]interface{}{}),
		[]string{},
		handleMagentoGetMediaImportStatus,
	)
}

type importFailedItem struct {
	Row   int    `json:"row"`
	SKU   string `json:"sku"`
	Error string `json:"error"`
}

func handleMagentoImportCreateProducts(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	filePath := strArg(m, "file_path")
	if filePath == "" {
		return mcp.ToolResultError("missing required argument 'file_path'")
	}

	data, ext, err := loadFileBytes(filePath)
	if err != nil {
		return mcp.ToolResultError(fmt.Sprintf("Failed to load file: %v", err))
	}

	rows, err := parseProductsFile(data, ext)
	if err != nil {
		return mcp.ToolResultError(fmt.Sprintf("Failed to parse file: %v", err))
	}

	if len(rows) == 0 {
		return mcp.ToolResultError("No data rows found in the provided file")
	}

	skipExisting := boolArg(m, "skip_existing", false)
	skipImages := boolArg(m, "skip_images", false)
	deferImages := boolArg(m, "defer_images", false)
	smartChunking := boolArg(m, "smart_chunking", true)

	totalRowsCount := len(rows)
	offset := intArg(m, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	limit := intArg(m, "limit", 0)
	if limit <= 0 {
		limit = intArg(m, "batch_size", 0)
	}

	if offset >= totalRowsCount {
		return map[string]interface{}{
			"success":         true,
			"total_rows":      totalRowsCount,
			"offset":          offset,
			"limit":           limit,
			"processed_count": 0,
			"has_more":        false,
			"next_offset":     0,
			"created_count":   0,
			"failed_count":    0,
			"failed_items":    []importFailedItem{},
			"warnings":        []string{fmt.Sprintf("offset %d is beyond total rows %d", offset, totalRowsCount)},
		}
	}

	var activeRows []ParsedProductRow
	var nextOffset int
	var hasMore bool

	if smartChunking && limit > 0 {
		activeRows, nextOffset, hasMore = alignSmartChunkBoundary(rows, offset, limit)
	} else {
		activeRows = rows[offset:]
		if limit > 0 && limit < len(activeRows) {
			activeRows = activeRows[:limit]
		}
		hasMore = (offset + len(activeRows)) < totalRowsCount
		if hasMore {
			nextOffset = offset + len(activeRows)
		}
	}

	// 1. Pre-check all SKUs in active rows in a single batch query (cuts 50% HTTP roundtrips)
	var allSKUs []string
	for _, r := range activeRows {
		if s := strings.TrimSpace(r.SKU); s != "" {
			allSKUs = append(allSKUs, s)
		}
	}
	checkCtx, checkCancel := context.WithTimeout(context.Background(), 30*time.Second)
	existingSKUs := checkExistingSKUs(checkCtx, client, allSKUs)
	checkCancel()

	// 2. Split active rows into simple products and configurable parent groups
	simpleRows, configGroups := groupConfigurableProducts(activeRows)

	// High throughput worker pool (default 6 workers)
	workerCount := 6
	if v := os.Getenv("MAGENTO_IMPORT_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workerCount = n
		}
	}

	type createResult struct {
		row      int
		sku      string
		err      error
		warnings []string
	}
	var results []createResult
	var resMu sync.Mutex

	var deferredTasks []MediaTask
	var defMu sync.Mutex

	resolver := newAttributeOptionResolver(client)

	// Phase 1: Create all simple / standalone products first
	if len(simpleRows) > 0 {
		jobs := make(chan ParsedProductRow, len(simpleRows))
		var wg sync.WaitGroup

		for w := 0; w < workerCount; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for row := range jobs {
					res, mTask := processCreateProductRow(client, resolver, row, existingSKUs, skipExisting, skipImages, deferImages)
					resMu.Lock()
					results = append(results, res)
					resMu.Unlock()
					if mTask != nil {
						defMu.Lock()
						deferredTasks = append(deferredTasks, *mTask)
						defMu.Unlock()
					}
				}
			}()
		}

		for _, row := range simpleRows {
			jobs <- row
		}
		close(jobs)
		wg.Wait()
	}

	// Phase 2: Create configurable parent products and automatically link child variations
	configurableLinks := make(map[string][]string) // parentSKU -> linkedChildSKUs
	var confMu sync.Mutex

	if len(configGroups) > 0 {
		confWorkers := 4
		if len(configGroups) < confWorkers {
			confWorkers = len(configGroups)
		}
		groupJobs := make(chan ConfigurableParentGroup, len(configGroups))
		var parentWg sync.WaitGroup

		for pw := 0; pw < confWorkers; pw++ {
			parentWg.Add(1)
			go func() {
				defer parentWg.Done()
				for group := range groupJobs {
					parentRes, pTask := processCreateProductRow(client, resolver, group.ParentRow, existingSKUs, skipExisting, skipImages, deferImages)
					if pTask != nil {
						defMu.Lock()
						deferredTasks = append(deferredTasks, *pTask)
						defMu.Unlock()
					}

					// If parent product was created or already existed (skipExisting), proceed to link options & children
					if parentRes.err == nil {
						ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
						linked, linkWarns, linkErr := linkConfigurableProduct(ctx, client, resolver, group.ParentRow, group.Children)
						cancel()

						if len(linkWarns) > 0 {
							parentRes.warnings = append(parentRes.warnings, linkWarns...)
						}
						if linkErr != nil {
							parentRes.warnings = append(parentRes.warnings, fmt.Sprintf("Configurable link warning: %v", linkErr))
						}
						if len(linked) > 0 {
							confMu.Lock()
							configurableLinks[group.ParentRow.SKU] = linked
							confMu.Unlock()
						}
					}

					resMu.Lock()
					results = append(results, parentRes)
					resMu.Unlock()
				}
			}()
		}

		for _, group := range configGroups {
			groupJobs <- group
		}
		close(groupJobs)
		parentWg.Wait()
	}

	// Phase 3: Enqueue deferred media uploads if defer_images is enabled
	queuedMediaCount := 0
	if deferImages && len(deferredTasks) > 0 {
		queuedMediaCount = globalMediaQueue.Enqueue(client, deferredTasks)
	}

	createdCount := 0
	var failedItems []importFailedItem
	var allWarnings []string

	for _, r := range results {
		if r.err != nil {
			failedItems = append(failedItems, importFailedItem{
				Row:   r.row,
				SKU:   r.sku,
				Error: r.err.Error(),
			})
		} else {
			createdCount++
		}
		if len(r.warnings) > 0 {
			allWarnings = append(allWarnings, r.warnings...)
		}
	}

	resp := map[string]interface{}{
		"success":            len(failedItems) == 0,
		"total_rows":         totalRowsCount,
		"offset":             offset,
		"limit":              limit,
		"processed_count":    len(activeRows),
		"has_more":           hasMore,
		"next_offset":        nextOffset,
		"created_count":      createdCount,
		"failed_count":       len(failedItems),
		"failed_items":       failedItems,
		"warnings":           allWarnings,
		"configurable_links": configurableLinks,
	}

	if deferImages {
		resp["media_deferred"] = true
		resp["media_queued_count"] = queuedMediaCount
		resp["media_status_hint"] = "Images are downloading and uploading in the background. Call tool 'magento_get_media_import_status' to monitor progress."
	}
	if skipImages {
		resp["images_skipped"] = true
	}

	return resp
}

// checkExistingSKUs queries Magento in batches of up to 150 SKUs using searchCriteria condition_type=in.
// It returns a map of uppercase SKU -> true for existing products.
func checkExistingSKUs(ctx context.Context, client *magento.Client, skus []string) map[string]bool {
	existing := make(map[string]bool)
	if len(skus) == 0 {
		return existing
	}

	batchSize := 150
	for i := 0; i < len(skus); i += batchSize {
		end := i + batchSize
		if end > len(skus) {
			end = len(skus)
		}
		chunk := skus[i:end]

		params := url.Values{}
		params.Set("searchCriteria[filter_groups][0][filters][0][field]", "sku")
		params.Set("searchCriteria[filter_groups][0][filters][0][value]", strings.Join(chunk, ","))
		params.Set("searchCriteria[filter_groups][0][filters][0][condition_type]", "in")
		params.Set("searchCriteria[pageSize]", strconv.Itoa(len(chunk)))
		params.Set("fields", "items[sku]")

		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		resp, status, err := client.Request(reqCtx, http.MethodGet, "/products", params, nil)
		cancel()

		if err == nil && status == http.StatusOK {
			var result struct {
				Items []struct {
					SKU string `json:"sku"`
				} `json:"items"`
			}
			if json.Unmarshal(resp, &result) == nil {
				for _, it := range result.Items {
					existing[strings.ToUpper(strings.TrimSpace(it.SKU))] = true
				}
			}
		}
	}

	return existing
}

func processCreateProductRow(client *magento.Client, resolver *attributeOptionResolver, row ParsedProductRow, existingSKUs map[string]bool, skipExisting bool, skipImages bool, deferImages bool) (struct {
	row      int
	sku      string
	err      error
	warnings []string
}, *MediaTask) {
	sku := strings.TrimSpace(row.SKU)
	if sku == "" {
		return struct {
			row      int
			sku      string
			err      error
			warnings []string
		}{row: row.RowNumber, sku: "", err: fmt.Errorf("missing required field 'sku'")}, nil
	}

	name := strings.TrimSpace(row.Name)
	if name == "" {
		return struct {
			row      int
			sku      string
			err      error
			warnings []string
		}{row: row.RowNumber, sku: sku, err: fmt.Errorf("missing required field 'name' for new product")}, nil
	}

	isConfigurable := strings.EqualFold(row.TypeID, "configurable")
	if (row.Price == nil || *row.Price < 0) && !isConfigurable {
		return struct {
			row      int
			sku      string
			err      error
			warnings []string
		}{row: row.RowNumber, sku: sku, err: fmt.Errorf("missing or invalid required field 'price' for new product")}, nil
	}

	price := 0.0
	if row.Price != nil {
		price = *row.Price
	}

	skuUpper := strings.ToUpper(sku)
	if existingSKUs[skuUpper] {
		if skipExisting {
			var mTask *MediaTask
			if deferImages && (row.ImageURL != "" || row.AdditionalImages != "" || row.VideoURL != "") {
				mTask = &MediaTask{
					SKU:              sku,
					MainImageURL:     row.ImageURL,
					AdditionalImages: row.AdditionalImages,
					VideoURL:         row.VideoURL,
					VideoTitle:       row.VideoTitle,
				}
			}
			return struct {
				row      int
				sku      string
				err      error
				warnings []string
			}{row: row.RowNumber, sku: sku, err: nil, warnings: []string{fmt.Sprintf("SKU '%s' already exists; skipped creation", sku)}}, mTask
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 2. Build product payload
	product := map[string]interface{}{
		"sku":   sku,
		"name":  name,
		"price": price,
	}

	if row.AttributeSetID != nil {
		product["attribute_set_id"] = *row.AttributeSetID
	} else {
		product["attribute_set_id"] = 4
	}

	if row.TypeID != "" {
		product["type_id"] = row.TypeID
	} else {
		product["type_id"] = "simple"
	}

	if row.Status != nil {
		product["status"] = *row.Status
	} else {
		product["status"] = 1
	}

	if row.Visibility != nil {
		product["visibility"] = *row.Visibility
	} else {
		product["visibility"] = 4
	}

	if row.Weight != nil {
		product["weight"] = *row.Weight
	} else {
		product["weight"] = 0
	}

	// Extension Attributes: stock and categories
	extAttrs := map[string]interface{}{}
	if row.Qty != nil || row.IsInStock != nil || row.ManageStock != nil {
		stockItem := map[string]interface{}{}
		if row.Qty != nil {
			stockItem["qty"] = *row.Qty
		}
		if row.IsInStock != nil {
			stockItem["is_in_stock"] = (*row.IsInStock == 1)
		} else if row.Qty != nil && *row.Qty > 0 {
			stockItem["is_in_stock"] = true
		}
		if row.ManageStock != nil {
			stockItem["manage_stock"] = (*row.ManageStock == 1)
		}
		extAttrs["stock_item"] = stockItem
	}

	if row.Categories != "" {
		catLinks := []map[string]interface{}{}
		for _, rawID := range strings.Split(row.Categories, ",") {
			idStr := strings.TrimSpace(rawID)
			if idStr != "" {
				catLinks = append(catLinks, map[string]interface{}{
					"position":    0,
					"category_id": idStr,
				})
			}
		}
		if len(catLinks) > 0 {
			extAttrs["category_links"] = catLinks
		}
	}
	if len(extAttrs) > 0 {
		product["extension_attributes"] = extAttrs
	}

	// Custom Attributes
	var customAttrs []map[string]interface{}
	addCustom := func(code string, val interface{}) {
		if val != nil && val != "" {
			customAttrs = append(customAttrs, map[string]interface{}{
				"attribute_code": code,
				"value":          val,
			})
		}
	}

	if row.ShortDescription != "" {
		addCustom("short_description", row.ShortDescription)
	}
	if row.Description != "" {
		addCustom("description", row.Description)
	}
	if row.SpecialPrice != nil {
		addCustom("special_price", fmt.Sprintf("%.2f", *row.SpecialPrice))
	}
	if row.SpecialFromDate != "" {
		addCustom("special_from_date", normalizeImportDate(row.SpecialFromDate))
	}
	if row.SpecialToDate != "" {
		addCustom("special_to_date", normalizeImportDate(row.SpecialToDate))
	}
	if row.Cost != nil {
		addCustom("cost", fmt.Sprintf("%.2f", *row.Cost))
	}
	if row.MSRP != nil {
		addCustom("msrp", fmt.Sprintf("%.2f", *row.MSRP))
	}
	if row.TaxClassID != nil {
		addCustom("tax_class_id", strconv.Itoa(*row.TaxClassID))
	}
	if row.URLKey != "" {
		addCustom("url_key", row.URLKey)
	}
	if row.MetaTitle != "" {
		addCustom("meta_title", row.MetaTitle)
	}
	if row.MetaKeywords != "" {
		addCustom("meta_keyword", row.MetaKeywords)
	}
	if row.MetaDescription != "" {
		addCustom("meta_description", row.MetaDescription)
	}

	for k, v := range row.CustomAttributes {
		resolved := resolver.Resolve(ctx, k, v)
		addCustom(k, resolved)
	}

	if len(customAttrs) > 0 {
		product["custom_attributes"] = customAttrs
	}

	// 3. Create Product
	_, _, err := client.Request(ctx, http.MethodPost, "/products", url.Values{}, map[string]interface{}{"product": product})
	if err != nil {
		return struct {
			row      int
			sku      string
			err      error
			warnings []string
		}{row: row.RowNumber, sku: sku, err: err}, nil
	}

	// 4. Upload Images and Video if present
	var warnings []string
	var mTask *MediaTask
	if row.ImageURL != "" || row.AdditionalImages != "" || row.VideoURL != "" {
		if deferImages {
			mTask = &MediaTask{
				SKU:              sku,
				MainImageURL:     row.ImageURL,
				AdditionalImages: row.AdditionalImages,
				VideoURL:         row.VideoURL,
				VideoTitle:       row.VideoTitle,
			}
		} else if !skipImages {
			warnings = uploadProductMedia(ctx, client, sku, row.ImageURL, row.AdditionalImages, row.VideoURL, row.VideoTitle)
		}
	}

	return struct {
		row      int
		sku      string
		err      error
		warnings []string
	}{row: row.RowNumber, sku: sku, err: nil, warnings: warnings}, mTask
}

func handleMagentoImportUpdateProducts(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	filePath := strArg(m, "file_path")
	if filePath == "" {
		return mcp.ToolResultError("missing required argument 'file_path'")
	}

	data, ext, err := loadFileBytes(filePath)
	if err != nil {
		return mcp.ToolResultError(fmt.Sprintf("Failed to load file: %v", err))
	}

	rows, err := parseProductsFile(data, ext)
	if err != nil {
		return mcp.ToolResultError(fmt.Sprintf("Failed to parse file: %v", err))
	}

	totalRowsCount := len(rows)
	offset := intArg(m, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	limit := intArg(m, "limit", 0)
	if limit <= 0 {
		limit = intArg(m, "batch_size", 0)
	}

	if offset >= totalRowsCount {
		return map[string]interface{}{
			"success":         true,
			"total_rows":      totalRowsCount,
			"offset":          offset,
			"limit":           limit,
			"processed_count": 0,
			"has_more":        false,
			"next_offset":     0,
			"updated_count":   0,
			"failed_count":    0,
			"failed_items":    []importFailedItem{},
			"warnings":        []string{fmt.Sprintf("offset %d is beyond total rows %d", offset, totalRowsCount)},
		}
	}

	activeRows := rows[offset:]
	if limit > 0 && limit < len(activeRows) {
		activeRows = activeRows[:limit]
	}

	hasMore := (offset + len(activeRows)) < totalRowsCount
	nextOffset := 0
	if hasMore {
		nextOffset = offset + len(activeRows)
	}

	// Process updates with worker pool (default 6 workers)
	workerCount := 6
	if v := os.Getenv("MAGENTO_IMPORT_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			workerCount = n
		}
	}
	jobs := make(chan ParsedProductRow, len(activeRows))
	type updateResult struct {
		row      int
		sku      string
		err      error
		warnings []string
	}
	results := make(chan updateResult, len(activeRows))

	resolver := newAttributeOptionResolver(client)

	var wg sync.WaitGroup
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for row := range jobs {
				res := processUpdateProductRow(client, resolver, row)
				results <- res
			}
		}()
	}

	for _, row := range activeRows {
		jobs <- row
	}
	close(jobs)

	wg.Wait()
	close(results)

	updatedCount := 0
	var failedItems []importFailedItem
	var allWarnings []string

	for r := range results {
		if r.err != nil {
			failedItems = append(failedItems, importFailedItem{
				Row:   r.row,
				SKU:   r.sku,
				Error: r.err.Error(),
			})
		} else {
			updatedCount++
		}
		if len(r.warnings) > 0 {
			allWarnings = append(allWarnings, r.warnings...)
		}
	}

	return map[string]interface{}{
		"success":         len(failedItems) == 0,
		"total_rows":      totalRowsCount,
		"offset":          offset,
		"limit":           limit,
		"processed_count": len(activeRows),
		"has_more":        hasMore,
		"next_offset":     nextOffset,
		"updated_count":   updatedCount,
		"failed_count":    len(failedItems),
		"failed_items":    failedItems,
		"warnings":        allWarnings,
	}
}

func processUpdateProductRow(client *magento.Client, resolver *attributeOptionResolver, row ParsedProductRow) struct {
	row      int
	sku      string
	err      error
	warnings []string
} {
	sku := strings.TrimSpace(row.SKU)
	if sku == "" {
		return struct {
			row      int
			sku      string
			err      error
			warnings []string
		}{row: row.RowNumber, sku: "", err: fmt.Errorf("missing required field 'sku'")}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 1. Verify SKU exists
	_, status, _ := client.Request(ctx, http.MethodGet, "/products/"+url.PathEscape(sku), url.Values{}, nil)
	if status == http.StatusNotFound {
		return struct {
			row      int
			sku      string
			err      error
			warnings []string
		}{row: row.RowNumber, sku: sku, err: fmt.Errorf("product with SKU '%s' not found in Magento", sku)}
	}

	// 2. Build partial update payload (only modified fields)
	product := map[string]interface{}{}

	if row.Name != "" {
		product["name"] = row.Name
	}
	if row.Price != nil {
		product["price"] = *row.Price
	}
	if row.Status != nil {
		product["status"] = *row.Status
	}
	if row.Visibility != nil {
		product["visibility"] = *row.Visibility
	}
	if row.Weight != nil {
		product["weight"] = *row.Weight
	}

	// Extension Attributes: stock and categories
	extAttrs := map[string]interface{}{}
	if row.Qty != nil || row.IsInStock != nil || row.ManageStock != nil {
		stockItem := map[string]interface{}{}
		if row.Qty != nil {
			stockItem["qty"] = *row.Qty
		}
		if row.IsInStock != nil {
			stockItem["is_in_stock"] = (*row.IsInStock == 1)
		} else if row.Qty != nil && *row.Qty > 0 {
			stockItem["is_in_stock"] = true
		}
		if row.ManageStock != nil {
			stockItem["manage_stock"] = (*row.ManageStock == 1)
		}
		extAttrs["stock_item"] = stockItem
	}

	if row.Categories != "" {
		catLinks := []map[string]interface{}{}
		for _, rawID := range strings.Split(row.Categories, ",") {
			idStr := strings.TrimSpace(rawID)
			if idStr != "" {
				catLinks = append(catLinks, map[string]interface{}{
					"position":    0,
					"category_id": idStr,
				})
			}
		}
		if len(catLinks) > 0 {
			extAttrs["category_links"] = catLinks
		}
	}
	if len(extAttrs) > 0 {
		product["extension_attributes"] = extAttrs
	}

	// Custom Attributes
	var customAttrs []map[string]interface{}
	addCustom := func(code string, val interface{}) {
		if val != nil && val != "" {
			customAttrs = append(customAttrs, map[string]interface{}{
				"attribute_code": code,
				"value":          val,
			})
		}
	}

	if row.ShortDescription != "" {
		addCustom("short_description", row.ShortDescription)
	}
	if row.Description != "" {
		addCustom("description", row.Description)
	}
	if row.SpecialPrice != nil {
		addCustom("special_price", fmt.Sprintf("%.2f", *row.SpecialPrice))
	}
	if row.SpecialFromDate != "" {
		addCustom("special_from_date", normalizeImportDate(row.SpecialFromDate))
	}
	if row.SpecialToDate != "" {
		addCustom("special_to_date", normalizeImportDate(row.SpecialToDate))
	}
	if row.Cost != nil {
		addCustom("cost", fmt.Sprintf("%.2f", *row.Cost))
	}
	if row.MSRP != nil {
		addCustom("msrp", fmt.Sprintf("%.2f", *row.MSRP))
	}
	if row.TaxClassID != nil {
		addCustom("tax_class_id", strconv.Itoa(*row.TaxClassID))
	}
	if row.URLKey != "" {
		addCustom("url_key", row.URLKey)
	}
	if row.MetaTitle != "" {
		addCustom("meta_title", row.MetaTitle)
	}
	if row.MetaKeywords != "" {
		addCustom("meta_keyword", row.MetaKeywords)
	}
	if row.MetaDescription != "" {
		addCustom("meta_description", row.MetaDescription)
	}

	for k, v := range row.CustomAttributes {
		resolved := resolver.Resolve(ctx, k, v)
		addCustom(k, resolved)
	}

	if len(customAttrs) > 0 {
		product["custom_attributes"] = customAttrs
	}

	// 3. Put Product update
	_, _, err := client.Request(ctx, http.MethodPut, "/products/"+url.PathEscape(sku), url.Values{}, map[string]interface{}{"product": product})
	if err != nil {
		return struct {
			row      int
			sku      string
			err      error
			warnings []string
		}{row: row.RowNumber, sku: sku, err: err}
	}

	// 4. Upload Images and Video if specified
	var warnings []string
	if row.ImageURL != "" || row.AdditionalImages != "" || row.VideoURL != "" {
		warnings = uploadProductMedia(ctx, client, sku, row.ImageURL, row.AdditionalImages, row.VideoURL, row.VideoTitle)
	}

	return struct {
		row      int
		sku      string
		err      error
		warnings []string
	}{row: row.RowNumber, sku: sku, err: nil, warnings: warnings}
}
