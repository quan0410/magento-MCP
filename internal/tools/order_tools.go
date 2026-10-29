package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerOrderTools(add toolAdder) {
	add(
		"magento_list_orders",
		"List orders from Magento 2 using searchCriteria (pageSize, currentPage, status, customer_email, customer_id, increment_id, created_at_from, created_at_to).",
		baseProps(map[string]interface{}{
			"page_size":        map[string]interface{}{"type": "integer", "description": "Number of orders per page (default 20, max 100)"},
			"current_page":     map[string]interface{}{"type": "integer", "description": "Page number (default 1)"},
			"status":           map[string]interface{}{"type": "string", "description": "Filter by order status (e.g. pending, processing, complete, canceled). Supports single status, comma-separated statuses (e.g. 'pending,processing'), or an array of statuses."},
			"statuses":         map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional list of multiple order statuses to filter by (e.g. ['pending', 'processing']). Alternative to passing comma-separated statuses in 'status'."},
			"customer_email":   map[string]interface{}{"type": "string", "description": "Filter by customer email address"},
			"customer_id":      map[string]interface{}{"type": "string", "description": "Filter by customer entity ID"},
			"increment_id":     map[string]interface{}{"type": "string", "description": "Filter by human-readable order number (e.g. 000000001)"},
			"created_at_from":  map[string]interface{}{"type": "string", "description": "Orders created on or after date/time (YYYY-MM-DD HH:MM:SS or YYYY-MM-DD)"},
			"created_at_to":    map[string]interface{}{"type": "string", "description": "Orders created on or before date/time (YYYY-MM-DD HH:MM:SS or YYYY-MM-DD)"},
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
		"magento_verify_and_get_order",
		"Verify order ownership using order number (increment_id) and customer email or phone, then retrieve sanitized order status, shipping details, and items.",
		baseProps(map[string]interface{}{
			"order_number": map[string]interface{}{"type": "string", "description": "Order number / increment ID (e.g. 100000001 or #100000001)"},
			"verification": map[string]interface{}{"type": "string", "description": "Customer email address or phone number used when placing the order"},
		}),
		[]string{"order_number", "verification"},
		handleMagentoVerifyAndGetOrder,
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
	fb := newFilterBuilder(q, pageSize, currentPage)

	if statuses := stringSliceArg(m, "status", "statuses"); len(statuses) > 0 {
		fb.addIn("status", statuses)
	}
	if email := strArg(m, "customer_email"); email != "" {
		fb.add("customer_email", email, "eq")
	}
	if custID := strArg(m, "customer_id"); custID != "" {
		fb.add("customer_id", custID, "eq")
	}
	if incID := strArg(m, "increment_id"); incID != "" {
		fb.add("increment_id", incID, "eq")
	}
	if dateFrom := strArg(m, "created_at_from"); dateFrom != "" {
		fb.add("created_at", dateFrom, "gteq")
	}
	if dateTo := strArg(m, "created_at_to"); dateTo != "" {
		fb.add("created_at", dateTo, "lteq")
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

func handleMagentoVerifyAndGetOrder(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	orderNumber := strings.TrimSpace(strArg(m, "order_number"))
	verification := strings.TrimSpace(strArg(m, "verification"))
	if orderNumber == "" || verification == "" {
		return mcp.ToolResultError("order_number and verification (email or phone) are required")
	}

	// Clean order number (strip leading '#' and whitespace)
	orderNumber = strings.TrimLeft(orderNumber, "#")
	orderNumber = strings.TrimSpace(orderNumber)

	// Fetch order by increment_id
	q := url.Values{}
	fb := newFilterBuilder(q, 1, 1)
	fb.add("increment_id", orderNumber, "eq")

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/orders", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	var searchResp struct {
		Items      []map[string]interface{} 
		TotalCount int                      
	}
	if err := json.Unmarshal(data, &searchResp); err != nil {
		return mcp.ToolResultError("Failed to parse Magento orders response: " + err.Error())
	}
	if len(searchResp.Items) == 0 {
		return jsonResult(map[string]interface{}{
			"verified": false,
			"message":  fmt.Sprintf("Không tìm thấy đơn hàng với mã '%s'. Vui lòng kiểm tra lại mã đơn hàng.", orderNumber),
		})
	}

	order := searchResp.Items[0]

	// Extract candidate emails and phones from order
	candidateEmails := []string{}
	candidatePhones := []string{}

	if email, ok := order["customer_email"].(string); ok && email != "" {
		candidateEmails = append(candidateEmails, email)
	}

	checkAddress := func(addrObj interface{}) {
		if addr, ok := addrObj.(map[string]interface{}); ok {
			if phone, ok := addr["telephone"].(string); ok && phone != "" {
				candidatePhones = append(candidatePhones, phone)
			}
			if email, ok := addr["email"].(string); ok && email != "" {
				candidateEmails = append(candidateEmails, email)
			}
		}
	}

	checkAddress(order["billing_address"])

	if extAttrs, ok := order["extension_attributes"].(map[string]interface{}); ok {
		if shippingAssignments, ok := extAttrs["shipping_assignments"].([]interface{}); ok && len(shippingAssignments) > 0 {
			if firstAssignment, ok := shippingAssignments[0].(map[string]interface{}); ok {
				if shipping, ok := firstAssignment["shipping"].(map[string]interface{}); ok {
					checkAddress(shipping["address"])
				}
			}
		}
	}

	// Verify logic
	verified := false
	cleanDigits := func(s string) string {
		var b strings.Builder
		for _, r := range s {
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		res := b.String()
		if strings.HasPrefix(res, "84") && len(res) >= 10 {
			res = "0" + res[2:]
		}
		return res
	}

	vInputClean := cleanDigits(verification)
	vLower := strings.ToLower(verification)

	for _, em := range candidateEmails {
		if strings.EqualFold(strings.TrimSpace(em), vLower) {
			verified = true
			break
		}
	}

	if !verified && len(vInputClean) >= 7 {
		for _, ph := range candidatePhones {
			phClean := cleanDigits(ph)
			if phClean != "" && (phClean == vInputClean || strings.HasSuffix(phClean, vInputClean) || strings.HasSuffix(vInputClean, phClean)) {
				verified = true
				break
			}
		}
	}

	if !verified {
		return jsonResult(map[string]interface{}{
			"verified":     false,
			"order_number": orderNumber,
			"message":      "Thông tin xác thực (email hoặc số điện thoại) không khớp với đơn hàng này. Vì lý do bảo mật, vui lòng kiểm tra lại thông tin.",
		})
	}

	// Status label mapping
	status, _ := order["status"].(string)
	statusLabels := map[string]string{
		"pending":         "Chờ xử lý / Chờ thanh toán",
		"pending_payment": "Chờ khách thanh toán",
		"processing":      "Đang xử lý / Chuẩn bị hàng",
		"complete":        "Đã hoàn thành / Đã giao thành công",
		"closed":          "Đã đóng / Hoàn tiền",
		"canceled":        "Đã hủy",
		"holded":          "Tạm giữ",
	}
	statusLabel := statusLabels[strings.ToLower(status)]
	if statusLabel == "" {
		statusLabel = status
	}

	// Mask phone (e.g. 0901234567 -> 090***567)
	maskPhone := func(p string) string {
		p = strings.TrimSpace(p)
		if len(p) < 6 {
			return p
		}
		prefixLen := 3
		suffixLen := 3
		if len(p) <= 7 {
			prefixLen = 2
			suffixLen = 2
		}
		return p[:prefixLen] + strings.Repeat("*", len(p)-prefixLen-suffixLen) + p[len(p)-suffixLen:]
	}

	// Mask name (e.g. 'Nguyen Van A' -> 'Nguyen *** A')
	maskName := func(name string) string {
		parts := strings.Fields(name)
		if len(parts) <= 1 {
			if len(name) <= 2 {
				return name
			}
			return string([]rune(name)[0]) + "***"
		}
		for i := 1; i < len(parts)-1; i++ {
			parts[i] = "***"
		}
		return strings.Join(parts, " ")
	}

	shippingInfo := map[string]interface{}{}
	if shipDesc, ok := order["shipping_description"].(string); ok {
		shippingInfo["method"] = shipDesc
	}

	var recipientName, recipientPhone, city, region string
	if extAttrs, ok := order["extension_attributes"].(map[string]interface{}); ok {
		if shippingAssignments, ok := extAttrs["shipping_assignments"].([]interface{}); ok && len(shippingAssignments) > 0 {
			if firstAssignment, ok := shippingAssignments[0].(map[string]interface{}); ok {
				if shipping, ok := firstAssignment["shipping"].(map[string]interface{}); ok {
					if addr, ok := shipping["address"].(map[string]interface{}); ok {
						fn, _ := addr["firstname"].(string)
						ln, _ := addr["lastname"].(string)
						recipientName = strings.TrimSpace(fn + " " + ln)
						recipientPhone, _ = addr["telephone"].(string)
						city, _ = addr["city"].(string)
						region, _ = addr["region"].(string)
					}
				}
			}
		}
	}
	if recipientName == "" {
		if bill, ok := order["billing_address"].(map[string]interface{}); ok {
			fn, _ := bill["firstname"].(string)
			ln, _ := bill["lastname"].(string)
			recipientName = strings.TrimSpace(fn + " " + ln)
			recipientPhone, _ = bill["telephone"].(string)
			city, _ = bill["city"].(string)
			region, _ = bill["region"].(string)
		}
	}

	if recipientName != "" {
		shippingInfo["recipient_name"] = maskName(recipientName)
	}
	if recipientPhone != "" {
		shippingInfo["recipient_phone"] = maskPhone(recipientPhone)
	}
	dest := strings.TrimSpace(strings.Trim(city+", "+region, ", "))
	if dest != "" {
		shippingInfo["destination"] = dest
	}

	if tracks, ok := order["tracks"].([]interface{}); ok && len(tracks) > 0 {
		var trackList []map[string]string
		for _, t := range tracks {
			if trk, ok := t.(map[string]interface{}); ok {
				trackList = append(trackList, map[string]string{
					"title":        fmt.Sprint(trk["title"]),
					"carrier_code": fmt.Sprint(trk["carrier_code"]),
					"track_number": fmt.Sprint(trk["track_number"]),
				})
			}
		}
		if len(trackList) > 0 {
			shippingInfo["tracks"] = trackList
		}
	}

	var sanitizedItems []map[string]interface{}
	if rawItems, ok := order["items"].([]interface{}); ok {
		for _, itemObj := range rawItems {
			it, ok := itemObj.(map[string]interface{})
			if !ok {
				continue
			}
			sanitizedItems = append(sanitizedItems, map[string]interface{}{
				"name":        it["name"],
				"sku":         it["sku"],
				"qty_ordered": it["qty_ordered"],
				"price":       it["price"],
				"row_total":   it["row_total"],
			})
		}
	}

	return jsonResult(map[string]interface{}{
		"verified":      true,
		"order_number":  orderNumber,
		"status":        status,
		"status_label":  statusLabel,
		"created_at":    order["created_at"],
		"total_amount":  order["grand_total"],
		"currency_code": order["order_currency_code"],
		"shipping":      shippingInfo,
		"items":         sanitizedItems,
	})
}
