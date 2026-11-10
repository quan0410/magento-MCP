# magento_public_get_product

Retrieve full product details by SKU for guest shoppers without authentication via Magento 2 GraphQL. Includes description, image gallery, available sizes, colors, swatches, and stock status.

This tool is public and does not require authentication or API keys.

## Audience

| Who | Token |
|---|---|
| anonymous | none |

## Agent rules

- Call this tool when an unauthenticated guest shopper asks for details, sizes, colors, material, or stock of a specific product SKU.
- Does not require credentials_json or OAuth tokens.

## When to call

A customer asks to view product details, check size availability, view product pictures, or inspect fabric/material description.

## Parameters

| Name | Required | Type | Sample | Meaning |
|---|---|---|---|---|
| `sku` | yes | string | "WF-SK-013" | Product SKU to view |
| `store_url` | no | string | "https://store.com" | Optional store URL override |

## Success fields

| Name | Sample | Meaning |
|---|---|---|
| `products` | {"items": [{"sku": "WF-SK-013", "name": "...", "stock_status": "IN_STOCK"}]} | Full product details |
