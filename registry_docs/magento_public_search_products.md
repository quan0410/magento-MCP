# magento_public_search_products

Search and list products for guest shoppers without authentication via Magento 2 GraphQL. Supports keyword search, category filter, price range, and returns product attributes with website stock status (IN_STOCK / OUT_OF_STOCK).

This tool is public and does not require authentication or API keys.

## Audience

| Who | Token |
|---|---|
| anonymous | none |

## Agent rules

- Call this tool when an unauthenticated guest shopper asks to search, browse, or filter products by name, keyword, category, or price range.
- Does not require credentials_json or OAuth tokens.

## When to call

A customer asks to find or explore products ("Tìm áo blazer", "Có váy nào dưới 1 triệu không?", "Xem danh mục đồ công sở").

## Parameters

| Name | Required | Type | Sample | Meaning |
|---|---|---|---|---|
| `search` | no | string | "blazer" | Search keyword matching product name or description |
| `category_id` | no | string | "20" | Filter by category ID |
| `price_from` | no | number | 200000 | Minimum price filter |
| `price_to` | no | number | 1500000 | Maximum price filter |
| `page_size` | no | integer | 20 | Number of products per page (default 20, max 50) |
| `current_page` | no | integer | 1 | Page number |
| `store_url` | no | string | "https://store.com" | Optional store URL override |

## Success fields

| Name | Sample | Meaning |
|---|---|---|
| `products` | {"total_count": 10, "items": [...]} | Search results matching criteria |
