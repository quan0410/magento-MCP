package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerCreditMemoTools(add toolAdder) {
	add(
		"magento_list_credit_memos",
		"List credit memos (refunds) in Magento using searchCriteria (pageSize, currentPage, order_id).",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Page size (default 20)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Page number (default 1)"},
			"order_id":     map[string]interface{}{"type": "string", "description": "Filter credit memos by parent Order ID"},
		}),
		nil,
		handleMagentoListCreditMemos,
	)

	add(
		"magento_get_credit_memo",
		"Retrieve full credit memo (refund) details by Credit Memo ID.",
		baseProps(map[string]interface{}{
			"creditmemo_id": map[string]interface{}{"type": "string", "description": "Credit memo entity ID"},
		}),
		[]string{"creditmemo_id"},
		handleMagentoGetCreditMemo,
	)

	add(
		"magento_create_credit_memo",
		"Create a credit memo (refund) for an order in Magento.",
		baseProps(map[string]interface{}{
			"order_id": map[string]interface{}{"type": "string", "description": "Order entity ID to refund"},
			"notify":   map[string]interface{}{"type": "boolean", "description": "Whether to notify the customer via email (default true)"},
		}),
		[]string{"order_id"},
		handleMagentoCreateCreditMemo,
	)
}

func handleMagentoListCreditMemos(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	pageSize := intArg(m, "page_size", 20)
	currentPage := intArg(m, "current_page", 1)

	q := url.Values{}
	fb := newFilterBuilder(q, pageSize, currentPage)

	if orderID := strArg(m, "order_id"); orderID != "" {
		fb.add("order_id", orderID, "eq")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/creditmemos", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetCreditMemo(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	creditmemoID := strArg(m, "creditmemo_id")
	if creditmemoID == "" {
		return mcp.ToolResultError("creditmemo_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/creditmemo/%s", creditmemoID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoCreateCreditMemo(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	orderID := strArg(m, "order_id")
	if orderID == "" {
		return mcp.ToolResultError("order_id is required")
	}

	body := map[string]interface{}{
		"notify": boolArg(m, "notify", true),
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPost, fmt.Sprintf("/order/%s/refund", orderID), url.Values{}, body)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
