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

### Success Response
```json
{
  "success": true,
  "message": "Document uploaded successfully",
  "filename": "document.pdf",
  "size": 1234567
}
```

### Error Response
```json
{
  "success": false,
  "error": "Error message describing what went wrong"
}
```

## Status Endpoint

Check if Aviary is running and configured properly:

```shell
curl http://localhost:8000/api/status
```

Response:
```json
{
  "status": "ok",
  "version": "1.0.0",
  "multi_user": false,
  "authentication_required": true
}
```

## Integrations

### Third-Party Integrations

* [AWS SES](https://github.com/rmitchellscott/aviary-integration-ses) - Lambda to provide emailed PDFs/ePubs to Aviary.

### SMS Integration (Twilio)

You can use the webhook endpoint with Twilio to upload documents via SMS:

1. Configure a Twilio webhook to point to `/api/webhook`
2. Send SMS messages with URLs to documents
3. Aviary will automatically download and upload to your reMarkable

### Email Integration

Set up email-to-document workflows by:
1. Using a service like AWS SES to receive emails
2. Extract PDF attachments or URLs from email content
3. POST to the webhook endpoint

### Automation Tools

Integrate with automation platforms:
- **Zapier**: Create zaps that trigger document uploads
- **IFTTT**: Set up applets for automated document handling
- **Home Assistant**: Add document upload automation
- **Custom Scripts**: Use the API in your own scripts and applications

## Rate Limiting

Aviary implements basic rate limiting on API endpoints:
- Webhook uploads: Limited by download timeout and processing time
- Status checks: No specific limits

For high-volume usage, consider implementing your own rate limiting or batching uploads.

## Error Handling

Common error scenarios and responses:

### Invalid URL
```json
{
  "success": false,
  "error": "Failed to download document: invalid URL"
}
```

### Authentication Required
```json
{
  "success": false,
  "error": "Authentication required"
}
```

### Invalid Content Type
```json
{
  "success": false,
  "error": "Unsupported content type: text/html"
}
```

### reMarkable Connection Error
```json
{
  "success": false,
  "error": "Failed to upload to reMarkable: device not paired"
}
```

## Best Practices

1. **Always use HTTPS** in production for API requests
2. **Handle errors gracefully** and implement retry logic for transient failures
3. **Validate content** before uploading to avoid unnecessary processing
4. **Use appropriate timeouts** for long-running uploads
5. **Monitor API usage** if using multi-user mode with usage tracking
6. **Implement proper logging** in your applications for debugging