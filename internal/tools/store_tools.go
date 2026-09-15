package tools

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/datumbridge/magento-mcp/internal/mcp"
)

func registerStoreTools(add toolAdder) {
	add(
		"magento_list_store_views",
		"List all store views configured in Magento, including store code, website ID, and name.",
		baseProps(nil),
		nil,
		handleMagentoListStoreViews,
	)

	add(
		"magento_list_store_configs",
		"List store configurations and active locale/currency settings.",
		baseProps(map[string]interface{}{
			"store_codes": map[string]interface{}{"type": "string", "description": "Optional comma-separated store codes"},
		}),
		nil,
		handleMagentoListStoreConfigs,
	)
}

func handleMagentoListStoreViews(args json.RawMessage) map[string]interface{} {
	client, _, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/store/storeViews", url.Values{}, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}

func handleMagentoListStoreConfigs(args json.RawMessage) map[string]interface{} {
	client, m, err := clientFrom(args)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	q := url.Values{}
	if sc := strArg(m, "store_codes"); sc != "" {
		q.Set("storeCodes[0]", sc)
	}

	c, cancel := ctx()
	defer cancel()
	data, _, err := client.Request(c, http.MethodGet, "/store/storeConfigs", q, nil)
	if err != nil {
		return mcp.ToolResultError(err.Error())
	}
	return rawResult(data)
}
