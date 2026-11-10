# magento_get_related_products

Retrieve related, cross-sell, or up-sell products linked to a product SKU for outfit styling, bundle recommendations, or alternative suggestions.

The gateway injects `credentials_json` from the connected Magento 2 account. Do not invent a token, paste a secret, or pass `credentials_path` / `credentials_json`. If those fields appear in the schema, omit them.

## Audience

The MCP server decides this row. Weaver reads it on Setup and does not let an admin change it. Anonymous still runs only through a Digital Employee. Anonymous cannot use a connected account, so its token is `none`.

| Who | Token |
|---|---|
| anonymous | none |

## Agent rules

- Call this tool when a customer is inquiring about what goes well with a product (mix & match / outfit styling), similar products, or upgrades.
- Default `link_type` is `crosssell` for outfit and accessory pairing.
- Scope requirement: Ensure the Magento 2 Integration ACL role has sufficient permissions (`Magento_Catalog::products`).

## When to call

The customer asks for styling advice, outfit recommendations ("Áo này phối với gì?", "Gợi ý đồ mặc kèm"), or related items for a specific product SKU.

## Parameters

| Name | Required | Type | Sample | Related tool | Meaning |
|---|---|---|---|---|---|
| `sku` | yes | string | "SHIRT-WHT-M" | `magento_list_products` → `items[0].sku` | Product SKU to find linked products for |
| `link_type` | no | string | "crosssell" | — | Link type: crosssell (styling/bundle), related, or upsell |
| `limit` | no | integer | 4 | — | Maximum number of linked products to return |

## Success fields

| Name | Sample | Meaning |
|---|---|---|
| `source_sku` | "SHIRT-WHT-M" | The SKU queried |
| `link_type` | "crosssell" | Link type returned |
| `items` | [{"sku": "PANT-BLK", "name": "Pants", "price": 450000}] | List of linked product details |
| `total_count` | 1 | Number of linked products returned |
