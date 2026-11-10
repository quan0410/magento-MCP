# magento_public_guest_cart

Create an empty guest cart, add outfit items to a guest cart, or retrieve guest cart contents without authentication via Magento 2 GraphQL.

This tool is public and does not require authentication or API keys.

## Audience

| Who | Token |
|---|---|
| anonymous | none |

## Agent rules

- Call this tool when an unauthenticated guest shopper asks to create a cart, add items to cart, or view cart contents.
- Does not require credentials_json or OAuth tokens.

## When to call

A customer asks: "Thêm mẫu này vào giỏ hàng", "Tạo giỏ hàng mới", "Xem giỏ hàng của tôi".

## Parameters

| Name | Required | Type | Sample | Meaning |
|---|---|---|---|---|
| `action` | yes | string | "create" | Action: 'create', 'add', or 'get' |
| `cart_id` | no | string | "guest-cart-hash" | Guest cart ID (required for 'add' and 'get') |
| `sku` | no | string | "WF-SK-013" | Product SKU to add |
| `qty` | no | number | 1 | Quantity to add |
| `store_url` | no | string | "https://store.com" | Optional store URL override |

## Success fields

| Name | Sample | Meaning |
|---|---|---|
| `cart_id` | "guest-cart-hash" | Cart ID generated |
| `cart` | {...} | Cart contents |
