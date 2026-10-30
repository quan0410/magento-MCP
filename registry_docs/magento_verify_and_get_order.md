# magento_verify_and_get_order

Verify customer ownership of an order using increment ID and email or phone number, then retrieve safe order details.

The gateway injects `credentials_json` from the connected Magento 2 account. Do not invent a token, paste a secret, or pass `credentials_path` / `credentials_json`. If those fields appear in the schema, omit them.

## Audience

The MCP server decides this row. Weaver reads it on Setup and does not let an admin change it. Anonymous still runs only through a Digital Employee. Anonymous cannot use a connected account, so its token is `none`.

| Who | Token |
|---|---|
| external | required |

## Agent rules

- Call this tool when an end user or customer asks to check the status or details of their order.
- Always require both the order number (`order_number`) and verification info (`verification`: email or phone number).
- On `isError: true` or error result, inspect `content[0].text`.
- If `verified: false`, inform the customer politely that their verification information did not match the order records.
- Scope requirement: Ensure the Magento 2 Integration ACL role has sufficient permissions (`Magento_Sales::actions_view`).

## When to call

The customer asks to check order status, delivery progress, tracking number, or items in an order by providing order number and phone or email.

## Parameters

| Name | Required | Type | Sample | Related tool | Meaning |
|---|---|---|---|---|---|
| `order_number` | yes | string | "100000025" | — | Order increment ID / number displayed to customer |
| `verification` | yes | string | "0987654321" | — | Email address or phone number used when placing the order |

## Success fields

| Name | Sample | Meaning |
|---|---|---|
| `verified` | true | Whether verification succeeded |
| `order_number` | "100000025" | Order increment ID |
| `status` | "processing" | Current order status code |
| `status_label` | "Đang xử lý / Chuẩn bị hàng" | Human-readable order status in Vietnamese |
| `total_amount` | 550000 | Grand total monetary charge |
| `shipping` | {"method": "Flat Rate", "recipient_name": "Nguyen *** A"} | Masked shipping info |
| `items` | [{"name": "Shirt", "sku": "SHIRT-01", "qty_ordered": 1}] | List of items ordered |
