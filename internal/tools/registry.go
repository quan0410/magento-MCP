package tools

import (
	"github.com/datumbridge/magento-mcp/internal/mcp"
	registrydocs "github.com/datumbridge/magento-mcp/registry_docs"
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
		input := schema(props, required)
		if doc := registrydocs.Markdown(name); doc != "" {
			input["x-datumbridge-docs"] = doc
		}
		descs = append(descs, mcp.ToolDesc{
			Name:        name,
			Description: desc,
			InputSchema: input,
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
	registerPromotionTools(add)
	registerCreditMemoTools(add)
	registerCmsTools(add)
	registerProductMediaTools(add)
	registerCartTools(add)

	return descs, handlers
}
