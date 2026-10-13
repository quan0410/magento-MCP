package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerPromotionTools(add toolAdder) {
	add(
		"magento_list_sales_rules",
		"List discount sales rules and promotions in Magento using searchCriteria (pageSize, currentPage, name, is_active).",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Page size (default 20)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Page number (default 1)"},
			"name":         map[string]interface{}{"type": "string", "description": "Filter by sales rule name"},
			"is_active":    map[string]interface{}{"type": "boolean", "description": "Filter active (true) or inactive (false) sales rules"},
		}),
		nil,
		handleMagentoListSalesRules,
	)

	add(
		"magento_get_sales_rule",
		"Retrieve sales rule / promotion details by rule ID.",
		baseProps(map[string]interface{}{
			"rule_id": map[string]interface{}{"type": "string", "description": "Sales rule entity ID"},
		}),
		[]string{"rule_id"},
		handleMagentoGetSalesRule,
	)

	add(
		"magento_delete_coupon",
		"Delete a discount coupon by Coupon ID in Magento.",
		baseProps(map[string]interface{}{
			"coupon_id": map[string]interface{}{"type": "string", "description": "Coupon entity ID"},
		}),
		[]string{"coupon_id"},
		handleMagentoDeleteCoupon,
	)
}

func handleMagentoListSalesRules(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	pageSize := intArg(m, "page_size", 20)
	currentPage := intArg(m, "current_page", 1)

	q := url.Values{}
	fb := newFilterBuilder(q, pageSize, currentPage)

	if name := strArg(m, "name"); name != "" {
		fb.add("name", "%"+name+"%", "like")
	}
	if v, ok := m["is_active"]; ok && v != nil {
		active := boolArg(m, "is_active", true)
		val := "0"
		if active {
			val = "1"
		}
		fb.add("is_active", val, "eq")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/salesRules/search", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetSalesRule(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	ruleID := strArg(m, "rule_id")
	if ruleID == "" {
		return mcp.ToolResultError("rule_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/salesRules/%s", ruleID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoDeleteCoupon(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	couponID := strArg(m, "coupon_id")
	if couponID == "" {
		return mcp.ToolResultError("coupon_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodDelete, fmt.Sprintf("/coupons/%s", couponID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
