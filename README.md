# magento-mcp

Magento 2 MCP tool-server for DatumBridge Agent Kit. Exposes Magento 2 REST API resources as MCP tools over Streamable HTTP (`POST /mcp`), authenticated by 4-part OAuth 1.0a credential bundles injected from `datumbridge-integrations`.

## Features

- **42 MCP Tools** spanning Products, Orders, Comments, Customers, Stock/Inventory (MSI), Categories, Invoices, Shipments, Store Configurations, Promotions & Coupons, Credit Memos (Refunds), CMS Pages & Blocks, Product Media, and Carts.
- **Streamable HTTP MCP** transport (protocol version `2024-11-05`) with session management (`Mcp-Session-Id`).
- **OAuth 1.0a Request Signing (HMAC-SHA256 & HMAC-SHA1)**: Compliant RFC 5849 request signer supporting modern Magento 2.4.7+ / Mage-OS (HMAC-SHA256) with automatic fallback for older versions (HMAC-SHA1).
- **Standard Credential Contract**: Consumes injected `credentials_json` containing `base_url`, `consumer_key`, `consumer_secret`, `access_token`, and `access_token_secret`.
- **Zero-Storage Auth**: Never stores credentials locally.
- **Configurable Tool Filtering**: Enable specific tools via `MAGENTO_TOOLS` env var or expose full scope by default.

## Running Locally

```bash
cp .env.example .env
# Edit .env with your Magento 2 store credentials
go run ./cmd/api
```

## Running Tests

```bash
# Unit tests
go test -v ./...

# Live integration test against connected Magento store
go run ./cmd/test_live
```
