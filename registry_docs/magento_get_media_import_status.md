# magento_get_media_import_status

Check the progress and status of background (deferred) product image and video uploads in Magento 2.

The gateway injects `credentials_json` from the connected Magento 2 account. Do not invent a token, paste a secret, or pass `credentials_path` / `credentials_json`. If those fields appear in the schema, omit them.

## Audience

The MCP server decides this row. Weaver reads it on Setup and does not let an admin change it. Anonymous still runs only through a Digital Employee. Anonymous cannot use a connected account, so its token is `none`.

| Who | Token |
|---|---|
| internal | required |

## Agent rules

- Call this tool to monitor the background media upload queue after calling `magento_import_create_products` with `defer_images: true`.
- Returns total queued tasks, completed, failed, pending counts, and any recent download/upload warnings.
- On `isError: true` or error result, inspect `content[0].text`.

## When to call

The user asks to check the progress or completion status of background media/image imports in Magento 2.

## Parameters

This tool takes no required parameters. Pass `{}`.

## Success fields

| Name | Sample | Meaning |
|---|---|---|
| `is_running` | false | Whether background media workers are currently downloading and uploading images |
| `total_queued` | 450 | Total number of media tasks enqueued |
| `completed` | 450 | Number of media tasks successfully uploaded to Magento |
| `failed` | 0 | Number of media tasks that failed |
| `pending` | 0 | Number of media tasks remaining in the queue |
| `recent_warnings` | [] | Recent media download or upload warnings |
| `last_started_at` | "2026-10-28T20:30:00Z" | Timestamp when the media queue started |
| `last_completed_at` | "2026-10-28T20:38:00Z" | Timestamp when the media queue finished |
