# magento_import_update_products

Bulk update existing products in Magento 2 from an Excel (.xlsx) file. Supports partial updates (only specified columns are modified, existing data is preserved). Required column: sku.

The gateway injects `credentials_json` from the connected Magento 2 account. Do not invent a token, paste a secret, or pass `credentials_path` / `credentials_json`. If those fields appear in the schema, omit them.

## Agent rules

- Call this tool only for the Magento 2 action in the title.
- Only Excel (.xlsx) files are supported; CSV files are not accepted.
- Ensure the input Excel file contains the required identifier column: `sku`.
- `file_path` is required and must point to a valid .xlsx file.
- This tool performs partial updates: only populated columns in the file are modified in Magento; omitted or empty cells leave current product fields untouched.
- If a product SKU does not exist in Magento, it is reported under `failed_items` and does not prevent other products from updating.
- Product images referenced in `image_url` and `additional_images` columns are fetched and uploaded asynchronously.
- On `isError: true` or error result, inspect `content[0].text`.
- Scope requirement: Ensure the Magento 2 Integration ACL role has sufficient permissions (`Magento_Catalog::products`).

## When to call

The user asks to update prices, stock quantities, descriptions, or images for existing Magento products using an Excel (.xlsx) file on disk.

## Parameters

| Name | Required | Type | Sample | Related tool | Meaning |
|---|---|---|---|---|---|
| `file_path` | yes | string | "/path/to/price_updates.xlsx" | — | Local file path to the Excel (.xlsx) file on disk or container |
| `offset` | no | integer | 0 | — | Zero-based row offset to start updating from (useful for chunked/batch updates of large files). Default 0 |
| `limit` | no | integer | 50 | — | Maximum number of rows to process in this batch. If 0 or omitted, processes all remaining rows. Prevents timeouts on large files |

**Related tool** (optional): when the user did not supply this id, Deep runs the named producer on the same MCP server and binds the output field (`tool` → `field`, machine-read as `x-datumbridge-related-tool` + `x-datumbridge-output-field` on the JSON Schema property). Leave `—` for free-text, enums, and values only the user can give.

## Success fields

| Name | Sample | Meaning |
|---|---|---|
| `total_rows` | 1000 | Total number of data rows parsed from the file |
| `offset` | 0 | Starting row index processed in this batch |
| `limit` | 50 | Batch size limit used |
| `processed_count` | 50 | Number of rows processed in this batch |
| `has_more` | true | Whether there are more rows remaining to process |
| `next_offset` | 50 | Offset to pass in the next batch request (0 if has_more is false) |
| `updated_count` | 48 | Number of products successfully updated |
| `failed_count` | 2 | Number of products that failed update |
| `failed_items` | [{"row": 14, "sku": "OLD-ITEM", "error": "Product not found"}] | Details of each failed row with reason |
| `warnings` | ["SKU 'XYZ' image download failed: HTTP 404"] | Non-fatal warnings such as failed image downloads |

## Error codes

When an error occurs, the Go MCP server returns the standard MCP Tool Error envelope:

```json
{
  "content": [
    {
      "type": "text",
      "text": "unsupported file format: only Excel (.xlsx) files are supported"
    }
  ],
  "isError": true
}
```

Deep Agent error classification and action reference table:

| `error_code` | retryable | When | What Deep Agent should do |
|---|---|---|---|
| `VALIDATION_ERROR` | false | file_path is missing or empty, or file format is not .xlsx. | Ensure file has .xlsx extension and valid Excel structure. |
| `NOT_FOUND` | false | Local file path does not exist on disk. | Verify file path before executing. |
| `AUTH_ERROR` | true | Magento HTTP 401: OAuth 1.0a token rejected. | Retry once after credential refresh. |

## Cases

### Case 1: Update prices and stock from local Excel file

```json
{
  "file_path": "/workspace/data/price_and_stock_updates.xlsx"
}
```
