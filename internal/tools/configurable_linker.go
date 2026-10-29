package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datumbridge/magento-mcp/internal/magento"
)

// ConfigurableVariationItem holds parsed info from a configurable_variations cell.
type ConfigurableVariationItem struct {
	ChildSKU   string
	Attributes map[string]string // e.g. "size": "S", "color": "Red"
}

// parseConfigurableVariations parses standard Magento format:
// "sku=CHILD-1,size=S,color=Red|sku=CHILD-2,size=M,color=Red"
func parseConfigurableVariations(raw string) []ConfigurableVariationItem {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	// Delimit variations by '|' or newline or semicolon
	var rawItems []string
	if strings.Contains(raw, "|") {
		rawItems = strings.Split(raw, "|")
	} else if strings.Contains(raw, "\n") {
		rawItems = strings.Split(raw, "\n")
	} else {
		rawItems = []string{raw}
	}

	var items []ConfigurableVariationItem
	for _, itemStr := range rawItems {
		itemStr = strings.TrimSpace(itemStr)
		if itemStr == "" {
			continue
		}

		item := ConfigurableVariationItem{
			Attributes: make(map[string]string),
		}

		pairs := strings.Split(itemStr, ",")
		for _, pair := range pairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := normalizeHeaderKey(kv[0])
			v := strings.TrimSpace(kv[1])
			if k == "sku" || k == "child_sku" {
				item.ChildSKU = v
			} else if k != "" && v != "" {
				item.Attributes[k] = v
			}
		}

		if item.ChildSKU != "" {
			items = append(items, item)
		}
	}

	return items
}

// parseConfigurableVariationLabels parses Magento format:
// "size=Size,color=Color" -> map[attributeCode]attributeLabel e.g. {"size": "Size", "color": "Color"}
func parseConfigurableVariationLabels(raw string) map[string]string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	res := make(map[string]string)
	pairs := strings.Split(raw, ",")
	for _, p := range pairs {
		kv := strings.SplitN(strings.TrimSpace(p), "=", 2)
		if len(kv) == 2 {
			k := normalizeHeaderKey(kv[0])
			v := strings.TrimSpace(kv[1])
			if k != "" && v != "" {
				res[k] = v
			}
		} else if len(kv) == 1 && strings.TrimSpace(kv[0]) != "" {
			k := normalizeHeaderKey(kv[0])
			res[k] = strings.Title(k)
		}
	}
	return res
}

// ConfigurableParentGroup pairs a parent configurable product with all its associated child simple products.
type ConfigurableParentGroup struct {
	ParentRow ParsedProductRow
	Children  []ParsedProductRow
}

// groupConfigurableProducts separates rows into simple/standalone rows and configurable parent groups.
func groupConfigurableProducts(rows []ParsedProductRow) ([]ParsedProductRow, []ConfigurableParentGroup) {
	var simples []ParsedProductRow
	var configurableRows []ParsedProductRow
	var groups []ConfigurableParentGroup

	for _, r := range rows {
		if strings.EqualFold(r.TypeID, "configurable") {
			configurableRows = append(configurableRows, r)
		} else {
			simples = append(simples, r)
		}
	}

	if len(configurableRows) == 0 {
		return simples, nil
	}

	// Map of SKU to row for quick lookup
	skuMap := make(map[string]ParsedProductRow)
	for _, s := range simples {
		skuMap[strings.ToUpper(strings.TrimSpace(s.SKU))] = s
	}

	// Track which children have been assigned to avoid duplicates
	assignedChildren := make(map[string]bool)

	for _, pRow := range configurableRows {
		parentSKU := strings.TrimSpace(pRow.SKU)
		group := ConfigurableParentGroup{
			ParentRow: pRow,
		}

		// 1. Gather children from configurable_variations column
		varItems := parseConfigurableVariations(pRow.ConfigurableVariations)
		for _, item := range varItems {
			cSKUUpper := strings.ToUpper(strings.TrimSpace(item.ChildSKU))
			var childRow ParsedProductRow
			if existing, ok := skuMap[cSKUUpper]; ok {
				childRow = existing
				// Merge any attributes from configurable_variations
				for k, v := range item.Attributes {
					if childRow.CustomAttributes == nil {
						childRow.CustomAttributes = make(map[string]interface{})
					}
					if _, has := childRow.CustomAttributes[k]; !has {
						childRow.CustomAttributes[k] = v
					}
				}
			} else {
				// Synthetic row if child wasn't in file
				childRow = ParsedProductRow{
					SKU:              item.ChildSKU,
					CustomAttributes: make(map[string]interface{}),
				}
				for k, v := range item.Attributes {
					childRow.CustomAttributes[k] = v
				}
			}

			if !assignedChildren[cSKUUpper] {
				group.Children = append(group.Children, childRow)
				assignedChildren[cSKUUpper] = true
			}
		}

		// 2. Intelligently match simple rows that belong to this parent by SKU prefix
		// Handles both "TSHIRT-RED-PARENT" -> "TSHIRT-RED-" and "TSHIRT-COTTON-RED" -> "TSHIRT-COTTON-RED-"
		parentUpper := strings.ToUpper(parentSKU)
		basePrefix := strings.TrimSuffix(parentUpper, "-PARENT")
		basePrefix = strings.TrimSuffix(basePrefix, "_PARENT")

		for _, s := range simples {
			cSKUUpper := strings.ToUpper(strings.TrimSpace(s.SKU))
			if assignedChildren[cSKUUpper] {
				continue
			}
			// Check if child SKU starts with parent prefix or stripped base prefix
			if strings.HasPrefix(cSKUUpper, basePrefix+"-") || strings.HasPrefix(cSKUUpper, basePrefix+"_") ||
				strings.HasPrefix(cSKUUpper, parentUpper+"-") || strings.HasPrefix(cSKUUpper, parentUpper+"_") {
				group.Children = append(group.Children, s)
				assignedChildren[cSKUUpper] = true
			}
		}

		groups = append(groups, group)
	}

	// 3. Positional fallback: if any configurable parent has 0 children, check rows following it
	for i, group := range groups {
		if len(group.Children) > 0 {
			continue
		}
		parentRowIdx := group.ParentRow.RowNumber
		// Collect simple rows located immediately after this parent until next configurable
		for _, s := range simples {
			if s.RowNumber > parentRowIdx {
				cSKUUpper := strings.ToUpper(strings.TrimSpace(s.SKU))
				if assignedChildren[cSKUUpper] {
					continue
				}
				// Stop if another parent was between them
				stopped := false
				for _, otherGroup := range groups {
					if otherGroup.ParentRow.RowNumber > parentRowIdx && otherGroup.ParentRow.RowNumber < s.RowNumber {
						stopped = true
						break
					}
				}
				if stopped {
					break
				}
				groups[i].Children = append(groups[i].Children, s)
				assignedChildren[cSKUUpper] = true
			}
		}
	}

	return simples, groups
}

// linkConfigurableProduct links configured options and child products to a configurable parent.
func linkConfigurableProduct(
	ctx context.Context,
	client *magento.Client,
	resolver *attributeOptionResolver,
	parentRow ParsedProductRow,
	children []ParsedProductRow,
) (linkedSKUs []string, warnings []string, err error) {
	parentSKU := strings.TrimSpace(parentRow.SKU)
	if len(children) == 0 {
		return nil, nil, fmt.Errorf("no child variations provided for configurable product '%s'", parentSKU)
	}

	// 1. Identify target variation attribute codes
	targetAttrLabels := parseConfigurableVariationLabels(parentRow.ConfigurableVariationLabels)
	varItems := parseConfigurableVariations(parentRow.ConfigurableVariations)

	// Non-variation custom attributes to skip
	skipAttrs := map[string]bool{
		"sku": true, "name": true, "price": true, "cost": true, "msrp": true,
		"special_price": true, "special_from_date": true, "special_to_date": true,
		"status": true, "visibility": true, "type_id": true, "attribute_set_id": true,
		"weight": true, "qty": true, "is_in_stock": true, "manage_stock": true,
		"short_description": true, "description": true, "categories": true,
		"tax_class_id": true, "url_key": true, "meta_title": true, "meta_keywords": true,
		"meta_description": true, "image_url": true, "additional_images": true,
		"video_url": true, "video_title": true, "configurable_variations": true,
		"configurable_variation_labels": true,
	}

	attrValues := make(map[string]map[int]bool) // attrCode -> set of value_index integers

	// Collect from children CustomAttributes
	for _, child := range children {
		for attrCode, rawVal := range child.CustomAttributes {
			normCode := strings.ToLower(strings.TrimSpace(attrCode))
			if skipAttrs[normCode] || rawVal == nil {
				continue
			}
			// If targetAttrLabels is explicitly specified, only configure those variation attributes
			if len(targetAttrLabels) > 0 {
				if _, ok := targetAttrLabels[normCode]; !ok {
					continue
				}
			}

			strVal := strings.TrimSpace(fmt.Sprint(rawVal))
			if strVal == "" {
				continue
			}

			valIdx, err := resolver.ResolveOptionInt(ctx, normCode, rawVal)
			if err != nil || valIdx <= 0 {
				continue
			}

			if attrValues[normCode] == nil {
				attrValues[normCode] = make(map[int]bool)
			}
			attrValues[normCode][valIdx] = true
		}
	}

	// Also collect from parsed ConfigurableVariations if any child was synthetic
	for _, item := range varItems {
		for k, v := range item.Attributes {
			normCode := strings.ToLower(strings.TrimSpace(k))
			if skipAttrs[normCode] || v == "" {
				continue
			}
			if len(targetAttrLabels) > 0 {
				if _, ok := targetAttrLabels[normCode]; !ok {
					continue
				}
			}
			valIdx, err := resolver.ResolveOptionInt(ctx, normCode, v)
			if err == nil && valIdx > 0 {
				if attrValues[normCode] == nil {
					attrValues[normCode] = make(map[int]bool)
				}
				attrValues[normCode][valIdx] = true
			}
		}
	}

	// If targetAttrLabels specified an attribute not yet in attrValues, ensure label is known
	for attrCode, lbl := range targetAttrLabels {
		if attrValues[attrCode] == nil {
			attrValues[attrCode] = make(map[int]bool)
		}
		_ = lbl
	}

	if len(attrValues) == 0 {
		warnings = append(warnings, fmt.Sprintf("Configurable product '%s': no variation attributes (like size/color) found on child products", parentSKU))
	}

	// 2. Fetch existing options on parent to support incremental option updates
	type existingOptionInfo struct {
		OptionID    int
		AttributeID int
		Values      map[int]bool
	}
	existingOptions := make(map[int]*existingOptionInfo) // attribute_id -> info

	optCtx, optCancel := context.WithTimeout(ctx, 15*time.Second)
	optResp, optStatus, optErr := client.Request(optCtx, http.MethodGet, "/configurable-products/"+url.PathEscape(parentSKU)+"/options/all", url.Values{}, nil)
	optCancel()

	if optErr == nil && optStatus == http.StatusOK {
		var opts []struct {
			ID          int    `json:"id"`
			AttributeID string `json:"attribute_id"`
			Values      []struct {
				ValueIndex int `json:"value_index"`
			} `json:"values"`
		}
		if json.Unmarshal(optResp, &opts) == nil {
			for _, o := range opts {
				if id, err := strconv.Atoi(o.AttributeID); err == nil && id > 0 {
					valMap := make(map[int]bool)
					for _, v := range o.Values {
						valMap[v.ValueIndex] = true
					}
					existingOptions[id] = &existingOptionInfo{
						OptionID:    o.ID,
						AttributeID: id,
						Values:      valMap,
					}
				}
			}
		}
	}

	// 3. Configure or update options on parent
	for attrCode, valSet := range attrValues {
		attrID, frontendLabel, err := resolver.GetAttributeInfo(ctx, attrCode)
		if err != nil || attrID <= 0 {
			warnings = append(warnings, fmt.Sprintf("Attribute '%s': could not retrieve metadata: %v", attrCode, err))
			continue
		}
		if customLbl, has := targetAttrLabels[attrCode]; has && customLbl != "" {
			frontendLabel = customLbl
		}

		existing, isExisting := existingOptions[attrID]
		if isExisting {
			// Check if any new value_index needs to be merged
			hasNewValues := false
			for vIdx := range valSet {
				if !existing.Values[vIdx] {
					existing.Values[vIdx] = true
					hasNewValues = true
				}
			}
			if !hasNewValues {
				// Option already has all current values
				continue
			}

			// Update option with merged values
			var allValues []map[string]interface{}
			for vIdx := range existing.Values {
				allValues = append(allValues, map[string]interface{}{"value_index": vIdx})
			}
			updatePayload := map[string]interface{}{
				"option": map[string]interface{}{
					"attribute_id":   strconv.Itoa(attrID),
					"label":          frontendLabel,
					"position":       0,
					"is_use_default": true,
					"values":         allValues,
				},
			}
			updCtx, updCancel := context.WithTimeout(ctx, 20*time.Second)
			_, _, _ = client.Request(updCtx, http.MethodPut, fmt.Sprintf("/configurable-products/%s/options/%d", url.PathEscape(parentSKU), existing.OptionID), url.Values{}, updatePayload)
			updCancel()
			continue
		}

		if len(valSet) == 0 {
			continue
		}

		var valuesList []map[string]interface{}
		var sortedIndices []int
		for idx := range valSet {
			sortedIndices = append(sortedIndices, idx)
		}
		sort.Ints(sortedIndices)
		for _, idx := range sortedIndices {
			valuesList = append(valuesList, map[string]interface{}{
				"value_index": idx,
			})
		}

		optionPayload := map[string]interface{}{
			"option": map[string]interface{}{
				"attribute_id":   strconv.Itoa(attrID),
				"label":          frontendLabel,
				"position":       0,
				"is_use_default": true,
				"values":         valuesList,
			},
		}

		addCtx, addCancel := context.WithTimeout(ctx, 20*time.Second)
		resp, status, err := client.Request(addCtx, http.MethodPost, "/configurable-products/"+url.PathEscape(parentSKU)+"/options", url.Values{}, optionPayload)
		addCancel()

		if err != nil || (status != http.StatusOK && status != http.StatusCreated) {
			if !strings.Contains(string(resp), "already added") {
				warnings = append(warnings, fmt.Sprintf("Failed to add option '%s' to '%s': status %d, resp: %s", frontendLabel, parentSKU, status, string(resp)))
			}
		}
	}

	// 4. Link child products to parent concurrently (high performance)
	workerNum := 4
	if workerNum > len(children) {
		workerNum = len(children)
	}

	var linkWg sync.WaitGroup
	var linkMu sync.Mutex
	childChan := make(chan string, len(children))

	for w := 0; w < workerNum; w++ {
		linkWg.Add(1)
		go func() {
			defer linkWg.Done()
			for childSKU := range childChan {
				childPayload := map[string]interface{}{
					"childSku": childSKU,
				}

				linkCtx, linkCancel := context.WithTimeout(ctx, 25*time.Second)
				resp, status, err := client.Request(linkCtx, http.MethodPost, "/configurable-products/"+url.PathEscape(parentSKU)+"/child", url.Values{}, childPayload)
				linkCancel()

				linkMu.Lock()
				if err != nil || (status != http.StatusOK && status != http.StatusCreated) {
					respStr := string(resp)
					if strings.Contains(respStr, "already attached") || strings.Contains(respStr, "already linked") {
						linkedSKUs = append(linkedSKUs, childSKU)
					} else {
						warnings = append(warnings, fmt.Sprintf("Failed to link child '%s' to '%s': status %d, resp: %s", childSKU, parentSKU, status, respStr))
					}
				} else {
					linkedSKUs = append(linkedSKUs, childSKU)
				}
				linkMu.Unlock()
			}
		}()
	}

	for _, child := range children {
		cSKU := strings.TrimSpace(child.SKU)
		if cSKU != "" {
			childChan <- cSKU
		}
	}
	close(childChan)
	linkWg.Wait()

	// 5. Ensure parent stock is marked as In Stock and Salable
	stkCtx, stkCancel := context.WithTimeout(ctx, 15*time.Second)
	stockPayload := map[string]interface{}{
		"product": map[string]interface{}{
			"sku": parentSKU,
			"extension_attributes": map[string]interface{}{
				"stock_item": map[string]interface{}{
					"is_in_stock":             true,
					"manage_stock":            true,
					"use_config_manage_stock": true,
				},
			},
		},
	}
	_, _, _ = client.Request(stkCtx, http.MethodPut, "/products/"+url.PathEscape(parentSKU), url.Values{}, stockPayload)
	stkCancel()

	return linkedSKUs, warnings, nil
}

// alignSmartChunkBoundary dynamically extends the chunk boundary so that no Configurable Parent-Child group
// is split across batches. If the proposed limit cuts off some children of a parent that was included in the batch,
// the boundary is extended to include all remaining children of that parent.
func alignSmartChunkBoundary(rows []ParsedProductRow, offset, limit int) (activeRows []ParsedProductRow, nextOffset int, hasMore bool) {
	total := len(rows)
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return nil, 0, false
	}
	if limit <= 0 || offset+limit >= total {
		return rows[offset:], 0, false
	}

	proposedEnd := offset + limit

	// Group the dataset to discover all parent-child associations
	_, groups := groupConfigurableProducts(rows)

	// Map each row index by SKU for fast lookup
	rowIdxBySKU := make(map[string]int)
	for idx, r := range rows {
		rowIdxBySKU[strings.ToUpper(strings.TrimSpace(r.SKU))] = idx
	}

	maxGroupIdx := make(map[int]int) // groupIndex -> max row index in rows
	groupOfRow := make(map[int]int)  // row index -> groupIndex

	for gIdx, g := range groups {
		pSKU := strings.ToUpper(strings.TrimSpace(g.ParentRow.SKU))
		maxIdx := -1
		if pIdx, ok := rowIdxBySKU[pSKU]; ok {
			groupOfRow[pIdx] = gIdx
			if pIdx > maxIdx {
				maxIdx = pIdx
			}
		}
		for _, c := range g.Children {
			cSKU := strings.ToUpper(strings.TrimSpace(c.SKU))
			if cIdx, ok := rowIdxBySKU[cSKU]; ok {
				groupOfRow[cIdx] = gIdx
				if cIdx > maxIdx {
					maxIdx = cIdx
				}
			}
		}
		maxGroupIdx[gIdx] = maxIdx
	}

	// Check if any row included in [offset, proposedEnd) belongs to a group whose members extend past proposedEnd
	adjustedEnd := proposedEnd
	for i := offset; i < adjustedEnd && i < total; i++ {
		if gIdx, inGroup := groupOfRow[i]; inGroup {
			if maxIdx, ok := maxGroupIdx[gIdx]; ok && maxIdx >= adjustedEnd {
				adjustedEnd = maxIdx + 1
			}
		}
	}

	if adjustedEnd > total {
		adjustedEnd = total
	}

	activeRows = rows[offset:adjustedEnd]
	hasMore = adjustedEnd < total
	if hasMore {
		nextOffset = adjustedEnd
	} else {
		nextOffset = 0
	}
	return activeRows, nextOffset, hasMore
}

