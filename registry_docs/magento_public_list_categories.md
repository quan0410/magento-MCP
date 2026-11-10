# magento_public_list_categories

Retrieve category hierarchy for guest shoppers to explore styles and catalog tree without authentication via Magento 2 GraphQL.

This tool is public and does not require authentication or API keys.

## Audience

| Who | Token |
|---|---|
| external | none |

## Agent rules

- Call this tool when an unauthenticated guest shopper asks to browse product categories or styles.
- Does not require credentials_json or OAuth tokens.

## When to call

A customer asks to view the shop's department or category tree ("Shop có những nhóm hàng nào?").

## Parameters

| Name | Required | Type | Sample | Meaning |
|---|---|---|---|---|
| `category_id` | no | string | "2" | Parent category ID |
| `store_url` | no | string | "https://store.com" | Optional store URL override |

## Success fields

| Name | Sample | Meaning |
|---|---|---|
| `categoryList` | [...] | Category hierarchy |
