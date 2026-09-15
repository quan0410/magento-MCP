# MCP Tool Specification — magento-mcp

| Field | Value |
|---|---|
| Folder | `magento-mcp` |
| mcpServer id | `magento-mcp` |
| Transport | Streamable HTTP `POST /mcp` |
| Protocol | `2024-11-05` |
| Health | `GET /health` |
| Default port | `8014` |
| Upstream | Magento 2 REST API (`/rest/V1/...`) |

## Tool visibility

Full scope by default (27 tools). `MAGENTO_TOOLS=tool1,tool2` narrows to an explicit allowlist.

## Tools (27)

| Name | Group | Method | Endpoint |
|---|---|---|---|
| `magento_list_products` | products | GET | `/products` |
| `magento_get_product` | products | GET | `/products/{sku}` |
| `magento_create_product` | products | POST | `/products` |
| `magento_update_product` | products | PUT | `/products/{sku}` |
| `magento_delete_product` | products | DELETE | `/products/{sku}` |
| `magento_list_product_types` | products | GET | `/products/types` |
| `magento_list_orders` | orders | GET | `/orders` |
| `magento_get_order` | orders | GET | `/orders/{id}` |
| `magento_add_order_comment` | orders | POST | `/orders/{id}/comments` |
| `magento_cancel_order` | orders | POST | `/orders/{id}/cancel` |
| `magento_hold_order` | orders | POST | `/orders/{id}/hold` (or `unhold`) |
| `magento_list_customers` | customers | GET | `/customers/search` |
| `magento_get_customer` | customers | GET | `/customers/{id}` |
| `magento_create_customer` | customers | POST | `/customers` |
| `magento_update_customer` | customers | PUT | `/customers/{id}` |
| `magento_get_stock_item` | inventory | GET | `/stockItems/{sku}` |
| `magento_update_stock_item` | inventory | PUT | `/products/{sku}/stockItems/{itemId}` |
| `magento_list_inventory_sources` | inventory | GET | `/inventory/sources` |
| `magento_list_categories` | categories | GET | `/categories` |
| `magento_get_category` | categories | GET | `/categories/{id}` |
| `magento_list_category_products` | categories | GET | `/categories/{id}/products` |
| `magento_assign_category_product` | categories | POST | `/categories/{id}/products` |
| `magento_list_invoices` | invoices_shipments | GET | `/invoices` |
| `magento_get_invoice` | invoices_shipments | GET | `/invoices/{id}` |
| `magento_create_shipment` | invoices_shipments | POST | `/order/{orderId}/ship` |
| `magento_list_store_views` | store | GET | `/store/storeViews` |
| `magento_list_store_configs` | store | GET | `/store/storeConfigs` |
