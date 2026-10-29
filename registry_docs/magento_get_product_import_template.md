# magento_get_product_import_template

Generate standard Excel (.xlsx) import template file with sample data and column definitions for creating or updating products in Magento 2.

The gateway injects `credentials_json` from the connected Magento 2 account. Do not invent a token, paste a secret, or pass `credentials_path` / `credentials_json`. If those fields appear in the schema, omit them.

## Agent rules

- Call this tool only for the Magento 2 action in the title. Do not guess missing IDs or SKUs.
- Use this tool before importing products to retrieve valid column headers, data formats, and sample rows in Excel (.xlsx) format.
- Missing required arguments are rejected by the MCP JSON-RPC schema (`-32602`) before the handler runs.
- On `isError: true` or error result, inspect `content[0].text`.
- Scope requirement: Ensure the Magento 2 Integration ACL role has sufficient permissions (`Magento_Catalog::products`).

## When to call

The user asks for a template, sample file, or guideline on how to prepare an Excel (.xlsx) file to import products into Magento 2.

## Parameters

| Name | Required | Type | Sample | Related tool | Meaning |
|---|---|---|---|---|---|
| `type` | no | string | "create" | — | Template type: 'create' for adding new products, or 'update' for modifying existing products (default 'create') |

**Related tool** (optional): when the user did not supply this id, Deep runs the named producer on the same MCP server and binds the output field (`tool` → `field`, machine-read as `x-datumbridge-related-tool` + `x-datumbridge-output-field` on the JSON Schema property). Leave `—` for free-text, enums, and values only the user can give.

## Success fields

| Name | Sample | Meaning |
|---|---|---|
| `format` | "xlsx" | File format generated ("xlsx") |
| `filename` | "magento_product_create_template.xlsx" | Recommended filename |
| `file_base64` | "UEsDBBQAAAAIA..." | Base64-encoded Microsoft Excel (.xlsx) workbook content |
| `fields_guide` | {"sku": "Product SKU..."} | Detailed dictionary explaining each supported field and formatting rules |

## Error codes

When an error occurs, the Go MCP server returns the standard MCP Tool Error envelope:

```json
{
  "content": [
    {
      "type": "text",
      "text": "Failed to generate Excel workbook: invalid template type"
    }
  ],
  "isError": true
}
```

Deep Agent error classification and action reference table:

| `error_code` | retryable | When | What Deep Agent should do |
|---|---|---|---|
| `VALIDATION_ERROR` | false | Invalid parameter format. | Use supported enum values: 'create' or 'update'. |
| `INTERNAL_ERROR` | false | Excel generation failure. | Retry tool call with type 'create'. |

## Cases

### Case 1: Retrieve create template as Excel (.xlsx)

```json
{
  "type": "create"
}
```

### Case 2: Retrieve update template as Excel (.xlsx)

```json
{
  "type": "update"
}
```
