# magento_import_create_products

Bulk import and create new products in Magento 2 from an Excel (.xlsx) file. Automatically downloads and uploads product images from URLs. Required columns: sku, name, price.

The gateway injects `credentials_json` from the connected Magento 2 account. Do not invent a token, paste a secret, or pass `credentials_path` / `credentials_json`. If those fields appear in the schema, omit them.

## Agent rules

- Call this tool only for the Magento 2 action in the title.
- Only Excel (.xlsx) files are supported; CSV files are not accepted.
- Ensure the input Excel file contains the minimum required columns: `sku`, `name`, `price`.
- `file_path` is required and must point to a valid .xlsx file.
- Products with SKUs that already exist in the Magento store are skipped to prevent overwriting existing data; use `magento_import_update_products` to modify existing products.
- Product images referenced in `image_url` and `additional_images` columns are fetched and uploaded asynchronously; image failures are returned in `warnings` and do not fail product creation.
- On `isError: true` or error result, inspect `content[0].text`.
- Scope requirement: Ensure the Magento 2 Integration ACL role has sufficient permissions (`Magento_Catalog::products`).

## When to call

The user asks to import or create multiple products from an Excel (.xlsx) file on disk into Magento 2.

## Parameters

| Name | Required | Type | Sample | Related tool | Meaning |
|---|---|---|---|---|---|
| `file_path` | yes | string | "/path/to/products.xlsx" | — | Local file path to the Excel (.xlsx) file on disk or container |
| `skip_existing` | no | boolean | true | — | If true, products whose SKU already exists in Magento will be skipped with a warning instead of failing the row. Useful when linking existing products or retrying partial imports. Default false |
| `skip_images` | no | boolean | false | — | If true, completely skips downloading and uploading product images and videos. Greatly speeds up product creation. Default false |
| `defer_images` | no | boolean | false | — | If true, products and configurable links are created immediately while images are queued and uploaded asynchronously in the background. Prevents client timeouts. Check progress via `magento_get_media_import_status`. Default false |
| `smart_chunking` | no | boolean | true | — | If true, automatically adjusts the batch boundary so that no Configurable Parent-Child group is split across batches. Default true |
| `offset` | no | integer | 0 | — | Zero-based row offset to start importing from (useful for chunked/batch imports of large files). Default 0 |
| `limit` | no | integer | 25 | — | Maximum number of rows to process in this batch (e.g. 25 or 50). If 0 or omitted, processes all remaining rows. Prevents timeouts on large files |

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
| `created_count` | 49 | Number of products successfully created |
| `failed_count` | 1 | Number of products that failed validation or creation |
| `failed_items` | [{"row": 5, "sku": "ABC", "error": "Price required"}] | Details of each failed row with reason |
| `warnings` | ["SKU 'XYZ' image download failed: HTTP 404"] | Non-fatal warnings such as failed image downloads |
| `configurable_links` | {"TSHIRT-RED-PARENT": ["TSHIRT-RED-S", "TSHIRT-RED-M"]} | Map of configurable parent SKUs to their linked child product SKUs |

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

### Case 1: Import products from local Excel file

```json
{
  "file_path": "/workspace/data/new_products.xlsx"
}
```
