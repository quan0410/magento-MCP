# magento_public_get_outfit

Retrieve outfit styling recommendations (cross-sell items), similar alternatives (up-sell items), and accessories (related items) for a product SKU without authentication via Magento 2 GraphQL.

This tool is public and does not require authentication or API keys.

## Audience

| Who | Token |
|---|---|
| anonymous | none |

## Agent rules

- Call this tool when an unauthenticated guest shopper asks for styling advice, outfit pairings, accessories, or similar items.
- Does not require credentials_json or OAuth tokens.

## When to call

A customer asks: "Áo này phối với quần hay túi xách nào?", "Gợi ý set đồ công sở với áo này", "Mẫu này hết size thì có mẫu nào tương tự không?".

## Parameters

| Name | Required | Type | Sample | Meaning |
|---|---|---|---|---|
| `sku` | yes | string | "BLZ-01" | Product SKU |
| `type` | no | string | "crosssell" | Type: crosssell, upsell, related, all |
| `store_url` | no | string | "https://store.com" | Optional store URL override |

## Success fields

| Name | Sample | Meaning |
|---|---|---|
| `crosssell_outfit` | [...] | Outfit pairing items |
| `upsell_alternatives` | [...] | Alternative similar items |
| `related_accessories` | [...] | Accessory items |
