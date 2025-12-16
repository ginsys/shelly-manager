# Operational Observability Enhancement

**Priority**: LOW - Enhancement
**Status**: not-started
**Effort**: 4-6 hours

## Context

API responses could include more operational metadata for monitoring and debugging. List endpoints should include version info and pagination metadata.

## Success Criteria

- [ ] `meta.version` added to list endpoints
- [ ] Pagination metadata standardized
- [ ] Log fields documented
- [ ] request_id propagation documented
- [ ] Structured logging enhanced

## Implementation

### Step 1: Add Meta Version to Responses

**File**: `internal/api/handlers.go`

Add version metadata to all list responses:

```go
type ListResponse struct {
    Data       interface{}       `json:"data"`
    Pagination *PaginationMeta   `json:"pagination,omitempty"`
    Meta       *ResponseMeta     `json:"meta"`
}

type ResponseMeta struct {
    Version   string    `json:"version"`
    RequestID string    `json:"request_id"`
    Timestamp time.Time `json:"timestamp"`
}

func (h *Handler) wrapListResponse(data interface{}, pagination *PaginationMeta, requestID string) ListResponse {
    return ListResponse{
        Data:       data,
        Pagination: pagination,
        Meta: &ResponseMeta{
            Version:   version.Version,
            RequestID: requestID,
            Timestamp: time.Now().UTC(),
        },
    }
}
```

### Step 2: Standardize Pagination Metadata

```go
type PaginationMeta struct {
    Page       int  `json:"page"`
    PageSize   int  `json:"page_size"`
    TotalItems int  `json:"total_items"`
    TotalPages int  `json:"total_pages"`
    HasNext    bool `json:"has_next"`
    HasPrev    bool `json:"has_prev"`
}
```

### Step 3: Document Log Fields

**File**: `docs/operations/logging.md`

Document standard log fields:
- `request_id` - Unique request identifier
- `method` - HTTP method
- `path` - Request path
- `status` - Response status code
- `duration_ms` - Request duration
- `user_agent` - Client user agent
- `client_ip` - Client IP address

### Step 4: Document Request ID Propagation

Show how request_id flows through the system:
1. Generated at middleware
2. Added to context
3. Included in all log entries
4. Returned in response headers
5. Included in error responses

## Validation

```bash
# Verify meta.version in responses
curl -s http://localhost:8080/api/v1/devices | jq '.meta.version'

# Verify request_id header
curl -I http://localhost:8080/api/v1/devices | grep X-Request-ID

# Verify structured logs
./bin/shelly-manager server 2>&1 | jq '.request_id'
```

## Dependencies

None

## Risk

Low - Additive changes only
