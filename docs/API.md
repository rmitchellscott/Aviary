# API Reference

This guide covers Aviary's webhook API and integration options for programmatic document uploads.

## Webhook Endpoint

The webhook endpoint (`/api/webhook`) supports two modes of operation for document uploads.

### URL-based uploads (Form data)

Upload documents by providing a URL to download from.

| Parameter                | Required? | Example | Description |
|--------------------------|-----------|---------|-------------|
| Body                     | Yes       | https://pdfobject.com/pdf/sample.pdf | URL to PDF/EPUB to download, or web article URL for extraction
| prefix                   | No        | Reports     | Folder and file-name prefix, only used if `manage` is also `true` |
| compress                 | No        | true/false  | Run Ghostscript compression (PDF only) |
| manage                   | No        | true/false  | Enable managed handling (renaming and cleanup) |
| archive                  | No        | true/false  | Download to PDF_DIR instead of /tmp |
| rm_dir                   | No        | Books       | Override default reMarkable upload directory |
| retention_days           | No        | 30          | Optional integer (in days) for cleanup if manage=true. Defaults to 7. |
| conflict_resolution      | No        | abort/overwrite/content_only | Override conflict resolution when file exists. Defaults to user/environment setting. |
| coverpage                | No        | current/first | Override coverpage setting for PDF uploads. Defaults to user/environment setting. |
| outputFormat             | No        | pdf/epub    | Output format for web articles, HTML, and Markdown. Defaults to CONVERSION_OUTPUT_FORMAT or pdf. |

### Document content uploads (JSON)

Upload documents by providing base64-encoded content directly.

| Parameter                | Required? | Example | Description |
|--------------------------|-----------|---------|-------------|
| body                     | Yes       | base64-encoded content | Base64 encoded document content |
| contentType              | No        | application/pdf | MIME type of the document |
| filename                 | No        | document.pdf | Original filename |
| isContent                | Yes       | true | Must be set to true for content uploads |
| prefix                   | No        | Reports | Folder and file-name prefix |
| compress                 | No        | true/false | Run Ghostscript compression (PDF only) |
| manage                   | No        | true/false | Enable managed handling |
| archive                  | No        | true/false | Save to PDF_DIR instead of /tmp |
| rm_dir                   | No        | Books | Override default reMarkable upload directory |
| retention_days           | No        | 30 | Optional integer (in days) for cleanup |
| conflict_resolution      | No        | abort/overwrite/content_only | Override conflict resolution when file exists |
| coverpage                | No        | current/first | Override coverpage setting for PDF uploads |
| outputFormat             | No        | pdf/epub | Output format for HTML and Markdown files. Defaults to CONVERSION_OUTPUT_FORMAT or pdf. |

**Supported content types:** PDF, JPEG, PNG, EPUB, Markdown (.md), HTML (.html)

**Note:** The `content_only` conflict resolution mode only works with PDF files. For non-PDF files (images, EPUB), it automatically falls back to `abort` behavior.

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

#### With conflict resolution and coverpage settings
```shell
curl -X POST http://localhost:8000/api/webhook \
  -d "Body=https://pdfobject.com/pdf/sample.pdf" \
  -d "conflict_resolution=overwrite" \
  -d "coverpage=first" \
  -d "compress=true"
```

#### With API key authentication
```shell
curl -X POST http://localhost:8000/api/webhook \
  -H "Authorization: Bearer your-api-key" \
  -d "Body=https://pdfobject.com/pdf/sample.pdf" \
  -d "compress=true" \
  -d "conflict_resolution=content_only"
```

#### Extract web article and convert to EPUB
```shell
curl -X POST http://localhost:8000/api/webhook \
  -H "Authorization: Bearer your-api-key" \
  -d "Body=https://example.com/article" \
  -d "outputFormat=epub" \
  -d "rm_dir=Articles"
```

**Note:** When a URL does not end with `.pdf` or `.epub`, Aviary will extract the readable article content (removing ads, navigation, etc.) using Mozilla's Readability algorithm and convert it to your chosen format (PDF or EPUB).

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
    "rm_dir": "Books",
    "conflict_resolution": "overwrite",
    "coverpage": "first"
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
    "compress": "false",
    "conflict_resolution": "abort"
  }'
```

#### Upload Markdown content and convert to PDF
```shell
curl -X POST http://localhost:8000/api/webhook \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-api-key" \
  -d '{
    "body": "IyBNeSBBcnRpY2xlCgpUaGlzIGlzIGEgKiptYXJrZG93bioqIGRvY3VtZW50Lg==",
    "contentType": "text/markdown",
    "filename": "article.md",
    "isContent": true,
    "outputFormat": "pdf",
    "rm_dir": "Notes"
  }'
```

**Note:** Markdown files with YAML frontmatter will have metadata (title, author, etc.) automatically extracted and used in the generated document.

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

## Backup and Restore API (Admin Only)

The backup and restore endpoints provide comprehensive data management capabilities for administrators. These endpoints are only available in multi-user mode and require admin authentication.

### Backup Operations

#### Create Backup Job
**POST** `/api/admin/backup-job`

Creates a background backup job that will be processed asynchronously.

**Query Parameters:**
| Parameter | Required? | Type | Description |
|-----------|-----------|------|-------------|
| include_files | No | boolean | Include user files in backup (default: true) |
| include_configs | No | boolean | Include configuration data (default: true) |
| user_ids | No | string | Comma-separated UUIDs of specific users to backup (all users if empty) |

**Response (202 Accepted):**
```json
{
  "success": true,
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "message": "Backup job created successfully"
}
```

#### List Backup Jobs
**GET** `/api/admin/backup-jobs`

Returns the 10 most recent backup jobs for the authenticated admin user, ordered by creation date (newest first).

**Response (200 OK):**
```json
{
  "jobs": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "admin_user_id": "660e8400-e29b-41d4-a716-446655440000",
      "status": "completed",
      "filename": "backup_2024-01-01.tar.gz",
      "file_size": 1024000,
      "storage_key": "backups/backup_2024-01-01.tar.gz",
      "error_message": null,
      "created_at": "2024-01-01T00:00:00Z",
      "completed_at": "2024-01-01T00:05:00Z"
    }
  ]
}
```

**Note:** To find the last successful backup, filter the results for `status: "completed"` - the first match is your most recent successful backup.

#### Get Backup Job Details
**GET** `/api/admin/backup-job/:id`

Returns details of a specific backup job.

**Response (200 OK):** Same structure as individual job in list response.

#### Download Backup
**GET** `/api/admin/backup-job/:id/download`

Downloads a completed backup file.

**Response:** Binary file download with `Content-Type: application/gzip`

#### Delete Backup Job
**DELETE** `/api/admin/backup-job/:id`

Deletes a backup job and its associated file.

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Backup job deleted successfully"
}
```

#### Analyze Backup File
**POST** `/api/admin/backup/analyze`

Analyzes a backup file before restore to validate its contents.

**Request:** Multipart form with field `backup_file` containing .tar.gz or .tgz file (max 32MB)

**Response (200 OK):**
```json
{
  "valid": true,
  "metadata": {
    "version": "1.0.0",
    "created_at": "2024-01-01T00:00:00Z",
    "user_count": 10,
    "file_count": 1000,
    "total_size": 1024000000,
    "includes_database": true,
    "includes_files": true,
    "database_tables": ["users", "files", "folders"],
    "errors": []
  }
}
```

### Restore Operations

#### Upload Restore File
**POST** `/api/admin/restore/upload`

Uploads a backup file for restoration. File is temporarily stored for 24 hours.

**Request:** Multipart form with field `backup_file` containing .tar.gz or .tgz file

**Response (200 OK):**
```json
{
  "upload_id": "770e8400-e29b-41d4-a716-446655440000",
  "filename": "backup_2024-01-01.tar.gz",
  "status": "uploaded",
  "file_size": 1024000,
  "expires_at": "2024-01-02T00:00:00Z",
  "message": "File uploaded successfully. Ready for restore."
}
```

#### List Restore Uploads
**GET** `/api/admin/restore/uploads`

Returns all pending restore uploads for the admin user.

**Response (200 OK):**
```json
{
  "uploads": [
    {
      "id": "770e8400-e29b-41d4-a716-446655440000",
      "admin_user_id": "660e8400-e29b-41d4-a716-446655440000",
      "filename": "backup_2024-01-01.tar.gz",
      "file_path": "/tmp/restore_770e8400_backup.tar.gz",
      "file_size": 1024000,
      "status": "uploaded",
      "expires_at": "2024-01-02T00:00:00Z",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

**Status values:** `uploaded`, `analyzed`, `extracting`, `extracted`, `failed`

#### Analyze Uploaded Restore File
**POST** `/api/admin/restore/uploads/:id/analyze`

Analyzes an already uploaded restore file.

**Response (200 OK):** Same as backup file analysis response.

#### Get Extraction Status
**GET** `/api/admin/restore/uploads/:id/extraction-status`

Gets the extraction progress for a restore upload.

**Response (200 OK):**
```json
{
  "status": "completed",
  "progress": 100,
  "error_message": null,
  "extraction_path": "/tmp/restore_extracted_770e8400"
}
```

#### Delete Restore Upload
**DELETE** `/api/admin/restore/uploads/:id`

Deletes a restore upload and its associated files.

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Restore upload deleted successfully"
}
```

#### Restore Database
**POST** `/api/admin/restore`

Initiates database restoration from an uploaded backup file.

**Request Body:**
```json
{
  "upload_id": "770e8400-e29b-41d4-a716-446655440000",
  "overwrite_files": false,
  "overwrite_database": false,
  "selected_user_ids": ["user-uuid-1", "user-uuid-2"]
}
```

**Parameters:**
| Parameter | Required? | Type | Description |
|-----------|-----------|------|-------------|
| upload_id | Yes | string | ID of the uploaded backup file |
| overwrite_files | No | boolean | Whether to overwrite existing files |
| overwrite_database | No | boolean | Whether to overwrite database tables |
| selected_user_ids | No | array | Specific users to restore (optional) |

**Response (200 OK):**
```json
{
  "success": true,
  "message": "Database restored successfully",
  "details": {
    "users_restored": 10,
    "files_restored": 1000,
    "folders_restored": 50
  }
}
```

### Example Backup/Restore Workflow

#### Creating a backup
```shell
# Create a backup job
curl -X POST http://localhost:8000/api/admin/backup-job \
  -H "Authorization: Bearer your-admin-api-key" \
  -d "include_files=true" \
  -d "include_configs=true"

# Check job status
curl http://localhost:8000/api/admin/backup-job/550e8400-e29b-41d4-a716-446655440000 \
  -H "Authorization: Bearer your-admin-api-key"

# Download completed backup
curl http://localhost:8000/api/admin/backup-job/550e8400-e29b-41d4-a716-446655440000/download \
  -H "Authorization: Bearer your-admin-api-key" \
  -o backup.tar.gz
```

#### Restoring from backup
```shell
# Upload backup file
curl -X POST http://localhost:8000/api/admin/restore/upload \
  -H "Authorization: Bearer your-admin-api-key" \
  -F "backup_file=@backup.tar.gz"

# Analyze uploaded file
curl -X POST http://localhost:8000/api/admin/restore/uploads/770e8400-e29b-41d4-a716-446655440000/analyze \
  -H "Authorization: Bearer your-admin-api-key"

# Perform restore
curl -X POST http://localhost:8000/api/admin/restore \
  -H "Authorization: Bearer your-admin-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "upload_id": "770e8400-e29b-41d4-a716-446655440000",
    "overwrite_files": true,
    "overwrite_database": false
  }'
```

## Rate Limiting

Aviary implements basic rate limiting on API endpoints:
- Webhook uploads: Limited by download timeout and processing time
- Status checks: No specific limits
- Backup/restore operations: Limited by processing time and storage

For high-volume usage, consider implementing your own rate limiting or batching uploads.
