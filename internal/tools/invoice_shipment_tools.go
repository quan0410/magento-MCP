package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/datumbridge/magento-mcp/internal/magento"
	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerInvoiceShipmentTools(add toolAdder) {
	add(
		"magento_list_invoices",
		"List invoices in Magento using searchCriteria with enriched customer, order, address, payment, and shipping details.",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Page size (default 20)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Page number (default 1)"},
			"order_id":     map[string]interface{}{"type": "string", "description": "Filter invoices by parent Order ID"},
			"enrich":       map[string]interface{}{"type": "boolean", "description": "Enrich invoice with customer, order, address and payment details (default true)"},
		}),
		nil,
		handleMagentoListInvoices,
	)

	add(
		"magento_get_invoice",
		"Retrieve full invoice details by Invoice ID, including enriched customer, order, address, and payment details.",
		baseProps(map[string]interface{}{
			"invoice_id": map[string]interface{}{"type": "string", "description": "Invoice entity ID"},
			"enrich":     map[string]interface{}{"type": "boolean", "description": "Enrich invoice with customer, order, address and payment details (default true)"},
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

func invoiceStateName(state interface{}) string {
	switch fmt.Sprintf("%v", state) {
	case "1":
		return "Open"
	case "2":
		return "Paid"
	case "3":
		return "Canceled"
	default:
		return fmt.Sprintf("%v", state)
	}
}

func enrichInvoice(ctx context.Context, client *magento.Client, inv map[string]interface{}, orderCache map[string]map[string]interface{}) {
	if inv == nil {
		return
	}

	if stateVal, ok := inv["state"]; ok {
		inv["state_name"] = invoiceStateName(stateVal)
	}

	orderIDVal, ok := inv["order_id"]
	if !ok || orderIDVal == nil {
		return
	}
	orderIDStr := fmt.Sprintf("%v", orderIDVal)
	if orderIDStr == "" || orderIDStr == "0" {
		return
	}

	var order map[string]interface{}
	if cached, ok := orderCache[orderIDStr]; ok {
		order = cached
	} else {
		oData, _, err := client.Request(ctx, http.MethodGet, fmt.Sprintf("/orders/%s", orderIDStr), url.Values{}, nil)
		if err == nil {
			var oMap map[string]interface{}
			if err := json.Unmarshal(oData, &oMap); err == nil {
				order = oMap
				orderCache[orderIDStr] = oMap
			}
		}
	}

	if order == nil {
		return
	}

	// 1. Order Information
	inv["order"] = map[string]interface{}{
		"order_id":       order["entity_id"],
		"increment_id":   order["increment_id"],
		"status":         order["status"],
		"state":          order["state"],
		"created_at":     order["created_at"],
		"store_name":     order["store_name"],
	}

	// 2. Customer / Account Information
	fname, _ := order["customer_firstname"].(string)
	lname, _ := order["customer_lastname"].(string)
	customerName := strings.TrimSpace(fname + " " + lname)
	inv["customer"] = map[string]interface{}{
		"customer_id":       order["customer_id"],
		"name":              customerName,
		"firstname":         fname,
		"lastname":          lname,
		"email":             order["customer_email"],
		"customer_group_id": order["customer_group_id"],
		"is_guest":          order["customer_is_guest"],
	}

	// 3. Billing Address
	if bAddr, ok := order["billing_address"].(map[string]interface{}); ok {
		inv["billing_address"] = bAddr
	}

	// 4. Shipping Address
	if ext, ok := order["extension_attributes"].(map[string]interface{}); ok {
		if saList, ok := ext["shipping_assignments"].([]interface{}); ok && len(saList) > 0 {
			if sa, ok := saList[0].(map[string]interface{}); ok {
				if ship, ok := sa["shipping"].(map[string]interface{}); ok {
					if addr, ok := ship["address"].(map[string]interface{}); ok {
						inv["shipping_address"] = addr
					}
				}
			}
		}
	}

	// 5. Payment Information
	if p, ok := order["payment"].(map[string]interface{}); ok {
		paymentInfo := map[string]interface{}{
			"method": p["method"],
		}
		if addInfo, ok := p["additional_information"].([]interface{}); ok && len(addInfo) > 0 {
			paymentInfo["title"] = addInfo[0]
		}
		inv["payment"] = paymentInfo
	}

	// 6. Shipping Information
	inv["shipping"] = map[string]interface{}{
		"method":               order["shipping_description"],
		"shipping_amount":      order["shipping_amount"],
		"base_shipping_amount": order["base_shipping_amount"],
	}

	// 7. Enrich invoice items with product type / metadata from order items
	if invItems, ok := inv["items"].([]interface{}); ok {
		orderItemsMap := make(map[string]map[string]interface{})
		if oItems, ok := order["items"].([]interface{}); ok {
			for _, oi := range oItems {
				if oiMap, ok := oi.(map[string]interface{}); ok {
					if itemID, ok := oiMap["item_id"]; ok {
						orderItemsMap[fmt.Sprintf("%v", itemID)] = oiMap
					}
				}
			}
		}

		for _, item := range invItems {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if orderItemID, ok := itemMap["order_item_id"]; ok {
					if oiMap, found := orderItemsMap[fmt.Sprintf("%v", orderItemID)]; found {
						if pType, ok := oiMap["product_type"]; ok {
							itemMap["product_type"] = pType
						}
					}
				}
			}
		}
	}
}

func handleMagentoListInvoices(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	pageSize := intArg(m, "page_size", 20)
	currentPage := intArg(m, "current_page", 1)
	enrich := boolArg(m, "enrich", true)

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

	if !enrich {
		return rawResult(data)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return rawResult(data)
	}

	orderCache := make(map[string]map[string]interface{})
	if items, ok := res["items"].([]interface{}); ok {
		for _, item := range items {
			if inv, ok := item.(map[string]interface{}); ok {
				enrichInvoice(c, client, inv, orderCache)
			}
		}
	}
	return jsonResult(res)
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
	enrich := boolArg(m, "enrich", true)

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/invoices/%s", invoiceID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	if !enrich {
		return rawResult(data)
	}

	var inv map[string]interface{}
	if err := json.Unmarshal(data, &inv); err != nil {
		return rawResult(data)
	}

	orderCache := make(map[string]map[string]interface{})
	enrichInvoice(c, client, inv, orderCache)
	return jsonResult(inv)
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
