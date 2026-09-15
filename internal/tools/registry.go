package tools

import (
	"github.com/datumbridge/magento-mcp/internal/mcp"
)

// Register returns the full magento-mcp tool catalogue: products, orders,
// customers, inventory, categories, invoices/shipments, and store configs.
// Full scope is the default; MAGENTO_TOOLS narrows it to an explicit allowlist if set.
func Register() ([]mcp.ToolDesc, map[string]mcp.ToolHandler) {
	cfg := LoadServerConfig()
	handlers := map[string]mcp.ToolHandler{}
	var descs []mcp.ToolDesc

	add := func(name, desc string, props map[string]interface{}, required []string, h mcp.ToolHandler) {
		if !cfg.toolEnabled(name) {
			return
		}
		descs = append(descs, mcp.ToolDesc{
			Name:        name,
			Description: desc,
			InputSchema: schema(props, required),
		})
		handlers[name] = h
	}

	registerProductTools(add)
	registerOrderTools(add)
	registerCustomerTools(add)
	registerInventoryTools(add)
	registerCategoryTools(add)
	registerInvoiceShipmentTools(add)
	registerStoreTools(add)

	return descs, handlers
}
