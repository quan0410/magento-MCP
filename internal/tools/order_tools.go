package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerOrderTools(add toolAdder) {
	add(
		"magento_list_orders",
		"List orders from Magento 2 using searchCriteria (pageSize, currentPage, status filter).",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Number of orders per page (default 20, max 100)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Page number (default 1)"},
			"status":       map[string]interface{}{"type": "string", "description": "Filter by order status (e.g. pending, processing, complete, canceled)"},
		}),
		nil,
		handleMagentoListOrders,
	)

	add(
		"magento_get_order",
		"Retrieve single order details by Order ID, including items, addresses, payment, and totals.",
		baseProps(map[string]interface{}{
			"order_id": map[string]interface{}{"type": "string", "description": "Order entity ID"},
		}),
		[]string{"order_id"},
		handleMagentoGetOrder,
	)

	add(
		"magento_add_order_comment",
		"Add a status history comment to an order with optional customer notification flag.",
		baseProps(map[string]interface{}{
			"order_id":        map[string]interface{}{"type": "string", "description": "Order entity ID"},
			"comment":         map[string]interface{}{"type": "string", "description": "Comment text to append to order history"},
			"status":          map[string]interface{}{"type": "string", "description": "Updated status string (optional)"},
			"is_customer_notified": map[string]interface{}{"type": "integer", "description": "1 if customer notified, 0 otherwise (default 0)"},
			"is_visible_on_front":  map[string]interface{}{"type": "integer", "description": "1 if visible in customer portal, 0 otherwise (default 0)"},
		}),
		[]string{"order_id", "comment"},
		handleMagentoAddOrderComment,
	)

	add(
		"magento_cancel_order",
		"Cancel an order in Magento by Order ID.",
		baseProps(map[string]interface{}{
			"order_id": map[string]interface{}{"type": "string", "description": "Order entity ID to cancel"},
		}),
		[]string{"order_id"},
		handleMagentoCancelOrder,
	)

	add(
		"magento_hold_order",
		"Put an order on hold (or unhold if action='unhold').",
		baseProps(map[string]interface{}{
			"order_id": map[string]interface{}{"type": "string", "description": "Order entity ID"},
			"action":   map[string]interface{}{"type": "string", "description": "hold or unhold (default hold)"},
		}),
		[]string{"order_id"},
		handleMagentoHoldOrder,
	)
}

func handleMagentoListOrders(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	pageSize := intArg(m, "page_size", 20)
	currentPage := intArg(m, "current_page", 1)

	q := url.Values{}
	q.Set("searchCriteria[pageSize]", strconv.Itoa(pageSize))
	q.Set("searchCriteria[currentPage]", strconv.Itoa(currentPage))

	if st := strArg(m, "status"); st != "" {
		q.Set("searchCriteria[filterGroups][0][filters][0][field]", "status")
		q.Set("searchCriteria[filterGroups][0][filters][0][value]", st)
		q.Set("searchCriteria[filterGroups][0][filters][0][conditionType]", "eq")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/orders", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetOrder(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	orderID := strArg(m, "order_id")
	if orderID == "" {
		return mcp.ToolResultError("order_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/orders/%s", orderID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoAddOrderComment(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	orderID := strArg(m, "order_id")
	comment := strArg(m, "comment")
	if orderID == "" || comment == "" {
		return mcp.ToolResultError("order_id and comment are required")
	}

	statusHistory := map[string]interface{}{
		"comment":              comment,
		"is_customer_notified": intArg(m, "is_customer_notified", 0),
		"is_visible_on_front":  intArg(m, "is_visible_on_front", 0),
	}
	if st := strArg(m, "status"); st != "" {
		statusHistory["status"] = st
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPost, fmt.Sprintf("/orders/%s/comments", orderID), url.Values{}, map[string]interface{}{"statusHistory": statusHistory})
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoCancelOrder(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	orderID := strArg(m, "order_id")
	if orderID == "" {
		return mcp.ToolResultError("order_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPost, fmt.Sprintf("/orders/%s/cancel", orderID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoHoldOrder(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	orderID := strArg(m, "order_id")
	if orderID == "" {
		return mcp.ToolResultError("order_id is required")
	}
	action := strArg(m, "action")
	endpoint := fmt.Sprintf("/orders/%s/hold", orderID)
	if action == "unhold" {
		endpoint = fmt.Sprintf("/orders/%s/unhold", orderID)
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPost, endpoint, url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
