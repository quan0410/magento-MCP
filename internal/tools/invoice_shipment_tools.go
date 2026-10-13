package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerInvoiceShipmentTools(add toolAdder) {
	add(
		"magento_list_invoices",
		"List invoices in Magento using searchCriteria (pageSize, currentPage, order_id).",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Page size (default 20)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Page number (default 1)"},
			"order_id":     map[string]interface{}{"type": "string", "description": "Filter invoices by parent Order ID"},
		}),
		nil,
		handleMagentoListInvoices,
	)

	add(
		"magento_get_invoice",
		"Retrieve full invoice details by Invoice ID.",
		baseProps(map[string]interface{}{
			"invoice_id": map[string]interface{}{"type": "string", "description": "Invoice entity ID"},
		}),
		[]string{"invoice_id"},
		handleMagentoGetInvoice,
	)

	add(
		"magento_create_shipment",
		"Create a shipment for an order to fulfill line items, with optional tracking numbers.",
		baseProps(map[string]interface{}{
			"order_id": map[string]interface{}{"type": "string", "description": "Order entity ID to ship"},
			"tracks":   map[string]interface{}{"type": "array", "description": "Tracking entries [{carrier_code, title, track_number}]"},
			"notify":   map[string]interface{}{"type": "boolean", "description": "Whether to notify the customer via email (default true)"},
		}),
		[]string{"order_id"},
		handleMagentoCreateShipment,
	)
}

func handleMagentoListInvoices(args json.RawMessage) map[string]interface{} {
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
	data, _, err := client.Request(c, http.MethodGet, "/invoices", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetInvoice(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	invoiceID := strArg(m, "invoice_id")
	if invoiceID == "" {
		return mcp.ToolResultError("invoice_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/invoices/%s", invoiceID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoCreateShipment(args json.RawMessage) map[string]interface{} {
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
	if tracks := sliceMapArg(m, "tracks"); len(tracks) > 0 {
		body["tracks"] = tracks
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPost, fmt.Sprintf("/order/%s/ship", orderID), url.Values{}, body)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
