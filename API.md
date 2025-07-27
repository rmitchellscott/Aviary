# API Reference

This guide covers Aviary's webhook API and integration options for programmatic document uploads.

## Webhook Endpoint

The webhook endpoint (`/api/webhook`) supports two modes of operation for document uploads.

### URL-based uploads (Form data)

Upload documents by providing a URL to download from.

| Parameter                | Required? | Example | Description |
|--------------------------|-----------|---------|-------------|
| Body                     | Yes       | https://pdfobject.com/pdf/sample.pdf | URL to PDF to download
| prefix                   | No        | Reports     | Folder and file-name prefix, only used if `manage` is also `true` |
| compress                 | No        | true/false  | Run Ghostscript compression |
| manage                   | No        | true/false  | Enable managed handling (renaming and cleanup) |
| archive                  | No        | true/false  | Download to PDF_DIR instead of /tmp |
| rm_dir                   | No        | Books       | Override default reMarkable upload directory |
| retention_days           | No        | 30          | Optional integer (in days) for cleanup if manage=true. Defaults to 7. |

### Document content uploads (JSON)

Upload documents by providing base64-encoded content directly.

| Parameter                | Required? | Example | Description |
|--------------------------|-----------|---------|-------------|
| body                     | Yes       | base64-encoded content | Base64 encoded document content |
| contentType              | No        | application/pdf | MIME type of the document |
| filename                 | No        | document.pdf | Original filename |
| isContent                | Yes       | true | Must be set to true for content uploads |
| prefix                   | No        | Reports | Folder and file-name prefix |
| compress                 | No        | true/false | Run Ghostscript compression |
| manage                   | No        | true/false | Enable managed handling |
| archive                  | No        | true/false | Save to PDF_DIR instead of /tmp |
| rm_dir                   | No        | Books | Override default reMarkable upload directory |
| retention_days           | No        | 30 | Optional integer (in days) for cleanup |

**Supported content types:** PDF, JPEG, PNG, EPUB

## Authentication

API requests can be authenticated using:
- **API Key (Header)**: `Authorization: Bearer your-api-key`
- **API Key (Alternative Header)**: `X-API-Key: your-api-key`
- **Session Cookie**: After web login (for same-origin requests)

### Single-User Mode
Set the `API_KEY` environment variable to enable API authentication.

### Multi-User Mode
Each user can generate multiple API keys through the web interface. API keys can have expiration dates and usage tracking.

## Example Requests

### URL-based uploads (Form data)

#### Basic request
```shell
curl -X POST http://localhost:8000/api/webhook \
  -d "Body=https://pdfobject.com/pdf/sample.pdf" \
  -d "prefix=Reports" \
  -d "compress=true" \
  -d "manage=true" \
  -d "rm_dir=Books"
```

#### With API key authentication
```shell
curl -X POST http://localhost:8000/api/webhook \
  -H "Authorization: Bearer your-api-key" \
  -d "Body=https://pdfobject.com/pdf/sample.pdf" \
  -d "compress=true"
```

### Document content uploads (JSON)

#### Upload base64-encoded PDF content
```shell
curl -X POST http://localhost:8000/api/webhook \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "body": "JVBERi0xLjQKJcOkw7zDtsO4DQo...",
    "contentType": "application/pdf",
    "filename": "document.pdf",
    "isContent": true,
    "compress": "true",
    "rm_dir": "Books"
  }'
```

#### Upload image content
```shell
curl -X POST http://localhost:8000/api/webhook \
  -H "Content-Type: application/json" \
  -d '{
    "body": "iVBORw0KGgoAAAANSUhEUgAAA...",
    "contentType": "image/png",
    "filename": "screenshot.png",
    "isContent": true,
    "compress": "false"
  }'
```

## Response Format

### Success Response (HTTP 202 Accepted)
```json
{
  "jobId": "550e8400-e29b-41d4-a716-446655440000"
}
```

The webhook endpoint returns a job ID immediately and processes the document asynchronously. Use the job ID to check the processing status via `/api/status/{jobId}`.

### Error Response (HTTP 4xx/5xx)
```json
{
  "error": "Error message describing what went wrong"
}
```

## Job Status Polling

After receiving a job ID from the webhook endpoint, use this endpoint to check the processing status:

```shell
curl http://localhost:8000/api/status/{jobId}
```

### Status Responses

#### Job Running
```json
{
  "status": "Running",
  "message": "backend.status.downloading",
  "progress": 25,
  "operation": "downloading"
}
```

#### Job Success
```json
{
  "status": "success", 
  "message": "backend.status.upload_success",
  "data": {
    "path": "Books/document.pdf"
  },
  "progress": 100,
  "operation": "uploading"
}
```

#### Job Error
```json
{
  "status": "error",
  "message": "backend.status.download_error", 
  "progress": 0,
  "operation": "downloading"
}
```

#### Job Not Found
```json
{
  "error": "backend.status.job_not_found"
}
```

### WebSocket Status Updates

For real-time updates, use the WebSocket endpoint:

```javascript
const ws = new WebSocket('ws://localhost:8000/api/status/ws/{jobId}');
ws.onmessage = (event) => {
  const job = JSON.parse(event.data);
  console.log('Job status:', job);
};
```

## Integrations

* [AWS SES](https://github.com/rmitchellscott/aviary-integration-ses) - Lambda to provide emailed PDFs/ePubs to Aviary.

## Rate Limiting

Aviary implements basic rate limiting on API endpoints:
- Webhook uploads: Limited by download timeout and processing time
- Status checks: No specific limits

For high-volume usage, consider implementing your own rate limiting or batching uploads.

## Error Handling

Common error scenarios and responses:

### Invalid JSON Format
```json
{
  "error": "Invalid JSON format"
}
```

### Authentication Required
```json
{
  "error": "Authentication required"
}
```
