package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerCustomerTools(add toolAdder) {
	add(
		"magento_list_customers",
		"Search and list customers in Magento 2 using searchCriteria (email, lastname, firstname, group_id, pageSize).",
		baseProps(map[string]interface{}{
			"page_size":    map[string]interface{}{"type": "integer", "description": "Page size (default 20)"},
			"current_page": map[string]interface{}{"type": "integer", "description": "Page number (default 1)"},
			"email":        map[string]interface{}{"type": "string", "description": "Filter by customer email"},
			"lastname":     map[string]interface{}{"type": "string", "description": "Filter by customer last name"},
			"firstname":    map[string]interface{}{"type": "string", "description": "Filter by customer first name"},
			"group_id":     map[string]interface{}{"type": "integer", "description": "Filter by customer group ID (e.g. 1 for General, 2 for Wholesale)"},
		}),
		nil,
		handleMagentoListCustomers,
	)

	add(
		"magento_get_customer",
		"Retrieve full customer account details by Customer ID.",
		baseProps(map[string]interface{}{
			"customer_id": map[string]interface{}{"type": "string", "description": "Customer entity ID"},
		}),
		[]string{"customer_id"},
		handleMagentoGetCustomer,
	)

	add(
		"magento_create_customer",
		"Create a new customer account in Magento with email, firstname, and lastname.",
		baseProps(map[string]interface{}{
			"email":     map[string]interface{}{"type": "string", "description": "Customer email"},
			"firstname": map[string]interface{}{"type": "string", "description": "Customer first name"},
			"lastname":  map[string]interface{}{"type": "string", "description": "Customer last name"},
			"group_id":  map[string]interface{}{"type": "integer", "description": "Customer group ID (default 1 for General)"},
		}),
		[]string{"email", "firstname", "lastname"},
		handleMagentoCreateCustomer,
	)

	add(
		"magento_update_customer",
		"Update customer details (firstname, lastname, email) by Customer ID.",
		baseProps(map[string]interface{}{
			"customer_id": map[string]interface{}{"type": "string", "description": "Customer entity ID"},
			"email":       map[string]interface{}{"type": "string", "description": "Updated email"},
			"firstname":   map[string]interface{}{"type": "string", "description": "Updated first name"},
			"lastname":    map[string]interface{}{"type": "string", "description": "Updated last name"},
		}),
		[]string{"customer_id"},
		handleMagentoUpdateCustomer,
	)
}

func handleMagentoListCustomers(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	pageSize := intArg(m, "page_size", 20)
	currentPage := intArg(m, "current_page", 1)

	q := url.Values{}
	fb := newFilterBuilder(q, pageSize, currentPage)

	if em := strArg(m, "email"); em != "" {
		fb.add("email", "%"+em+"%", "like")
	}
	if ln := strArg(m, "lastname"); ln != "" {
		fb.add("lastname", "%"+ln+"%", "like")
	}
	if fn := strArg(m, "firstname"); fn != "" {
		fb.add("firstname", "%"+fn+"%", "like")
	}
	if grpID := intArg(m, "group_id", -1); grpID >= 0 {
		fb.add("group_id", strconv.Itoa(grpID), "eq")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/customers/search", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoGetCustomer(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	customerID := strArg(m, "customer_id")
	if customerID == "" {
		return mcp.ToolResultError("customer_id is required")
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, fmt.Sprintf("/customers/%s", customerID), url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoCreateCustomer(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	email := strArg(m, "email")
	firstname := strArg(m, "firstname")
	lastname := strArg(m, "lastname")
	if email == "" || firstname == "" || lastname == "" {
		return mcp.ToolResultError("email, firstname, and lastname are required")
	}

	customer := map[string]interface{}{
		"email":     email,
		"firstname": firstname,
		"lastname":  lastname,
		"group_id":  intArg(m, "group_id", 1),
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPost, "/customers", url.Values{}, map[string]interface{}{"customer": customer})
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoUpdateCustomer(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	customerID := strArg(m, "customer_id")
	if customerID == "" {
		return mcp.ToolResultError("customer_id is required")
	}

	customer := map[string]interface{}{}
	if em := strArg(m, "email"); em != "" {
		customer["email"] = em
	}
	if fn := strArg(m, "firstname"); fn != "" {
		customer["firstname"] = fn
	}
	if ln := strArg(m, "lastname"); ln != "" {
		customer["lastname"] = ln
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodPut, fmt.Sprintf("/customers/%s", customerID), url.Values{}, map[string]interface{}{"customer": customer})
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
