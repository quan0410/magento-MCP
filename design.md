# Design — magento-mcp

## Class

**Tool-server** (Streamable HTTP MCP), matching `Bright-Data-MCP`, `jira-mcp`, and `shopify-mcp`.

## Language

Go 1.23, matching the platform standard established in `doc/MCP_TOOL_REFERENCE_ARCHITECTURE.md` (BR1).

## Upstream

- Magento 2 REST API (`{base_url}/rest/V1/...`)
- `{base_url}` comes directly from the injected credential bundle (`Credentials.BaseURL`), never from tool arguments (preventing multi-tenant confusion).

## Auth

- Method: One-legged OAuth 1.0a HMAC-SHA1 request signing (`Authorization: OAuth oauth_consumer_key=..., oauth_signature=...`).
- Production: Vault `credentials_json` injected per tool call via `POST /internal/v1/inject` on `datumbridge-integrations`.
- Fallback: `MAGENTO_BASE_URL`, `MAGENTO_CONSUMER_KEY`, `MAGENTO_CONSUMER_SECRET`, `MAGENTO_ACCESS_TOKEN`, `MAGENTO_ACCESS_TOKEN_SECRET` env vars (local dev only).
- `credentials_path` under `MAGENTO_CREDENTIALS_DIR` supported as a path-jailed alternative.
- Refresh: Magento Admin Integration tokens do not expire and do not use refresh tokens.

## Tools

42 tools across 12 categories:
1. **Products** (`internal/tools/product_tools.go`): 6 tools
2. **Orders** (`internal/tools/order_tools.go`): 5 tools
3. **Customers** (`internal/tools/customer_tools.go`): 4 tools
4. **Inventory & MSI** (`internal/tools/inventory_tools.go`): 3 tools
5. **Categories** (`internal/tools/category_tools.go`): 4 tools
6. **Invoices & Shipments** (`internal/tools/invoice_shipment_tools.go`): 3 tools
7. **Store Config** (`internal/tools/store_tools.go`): 2 tools
8. **Promotions & Coupons** (`internal/tools/promotion_tools.go`): 3 tools
9. **Credit Memos / Refunds** (`internal/tools/creditmemo_tools.go`): 3 tools
10. **CMS Pages & Blocks** (`internal/tools/cms_tools.go`): 4 tools
11. **Product Media** (`internal/tools/product_media_tools.go`): 2 tools
12. **Carts & Quotes** (`internal/tools/cart_tools.go`): 3 tools

## Security Baseline

- Never log secrets or tokens.
- Mask keys in logs and status outputs.
- Base URL pinned from credentials bundle.
- 10 MiB cap on upstream response bodies.
