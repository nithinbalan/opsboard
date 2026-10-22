# API contract

Base path: `/api/v1`

## Task representation

```json
{
  "id": 1,
  "title": "Prepare release",
  "description": "Build and validate the release",
  "status": "todo",
  "created_at": "2026-01-01T12:00:00Z",
  "updated_at": "2026-01-01T12:00:00Z"
}
```

Statuses: `todo`, `in_progress`, `done`. Create requires a non-empty title (maximum 200 characters). Description is limited to 5,000 characters. Unknown JSON fields are rejected. Request bodies are limited to 1 MiB.

Errors return JSON, for example:

```json
{"error":"task not found"}
```

Health endpoints are outside the API base path: `/health/live` and `/health/ready`.
