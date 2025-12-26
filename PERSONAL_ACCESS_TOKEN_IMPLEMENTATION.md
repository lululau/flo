# Personal Access Token Implementation

## Overview

This document describes the implementation of personal access token authentication for the Aliyun DevOps Pipelines TUI application, following the official Aliyun DevOps documentation.

## Implementation Details

### 1. Configuration Support

The application now supports personal access token authentication through the configuration file `~/.flo/config.yml`:

```yaml
# Recommended authentication method
organization_id: "your_organization_id"
personal_access_token: "your_personal_access_token"
endpoint: "openapi-rdc.aliyuncs.com"  # Optional, defaults to this value

# Fallback authentication method
# access_key_id: "your_access_key_id"
# access_key_secret: "your_access_key_secret"
# region_id: "cn-hangzhou"
```

### 2. Client Architecture Changes

The `Client` struct in `internal/api/client.go` has been enhanced to support dual authentication modes:

```go
type Client struct {
    sdkClient           *devops_rdc.Client // For AccessKey authentication
    httpClient          *http.Client       // For personal access token authentication
    endpoint            string             // API endpoint for token-based requests
    personalAccessToken string             // Personal access token
    useToken            bool               // Authentication method flag
}
```

### 3. Authentication Method Selection

The application automatically selects the authentication method based on configuration:

1. **Priority 1**: Personal Access Token (if `personal_access_token` is provided)
2. **Priority 2**: AccessKey (if `access_key_id` and `access_key_secret` are provided)

### 4. HTTP Client Implementation

For personal access token authentication, the application uses a custom HTTP client instead of the Aliyun SDK, because:

- The SDK's `BearerTokenCredential` doesn't work properly with DevOps API endpoints
- Personal access tokens require specific HTTP headers and endpoint handling
- Direct HTTP requests provide better control over the authentication flow

### 5. API Method Adaptation

The `ListPipelines` method now supports both authentication modes:

```go
func (c *Client) ListPipelines(organizationId string) ([]Pipeline, error) {
    if c.useToken {
        return c.listPipelinesWithToken(organizationId)
    }
    // Fall back to SDK-based implementation
    // ... existing SDK code
}
```

### 6. Token-based API Requests

The `makeTokenRequest` method handles HTTP requests with personal access token authentication:

```go
func (c *Client) makeTokenRequest(method, path string, body interface{}) (map[string]interface{}, error) {
    // Sets Authorization: Bearer <token> header
    // Handles JSON request/response parsing
    // Provides error handling for API responses
}
```

## Benefits of Personal Access Token Authentication

1. **Security**: More secure than AccessKey/Secret pairs
2. **Granular Permissions**: Can be scoped to specific operations
3. **Easy Revocation**: Can be revoked without affecting other credentials
4. **Recommended by Aliyun**: Official recommendation from Aliyun DevOps documentation
5. **No Region Management**: Endpoint-based, no need to manage regions

## Usage Instructions

### Obtaining a Personal Access Token

1. Visit: https://help.aliyun.com/zh/yunxiao/developer-reference/obtain-personal-access-token
2. Log into Aliyun DevOps Console
3. Navigate to Personal Settings → Access Tokens
4. Create a new token with appropriate permissions
5. Copy the token value to your configuration file

### Configuration Example

```yaml
# ~/.flo/config.yml
organization_id: "12345678"
personal_access_token: "your_actual_token_here"
endpoint: "openapi-rdc.aliyuncs.com"
```

### Running the Application

```bash
# Build the application
go build -o flo ./cmd/aliyun-pipelines-tui

# Run with personal access token authentication
./flo
```

## Fallback Support

The application maintains backward compatibility with AccessKey authentication:

```yaml
# ~/.flo/config.yml (fallback method)
organization_id: "12345678"
access_key_id: "your_access_key_id"
access_key_secret: "your_access_key_secret"
region_id: "cn-hangzhou"
```

## Error Handling

The implementation includes comprehensive error handling for:

- Invalid or expired tokens
- Network connectivity issues
- API endpoint resolution
- Response parsing errors
- Authentication failures

## Future Enhancements

1. **Token Refresh**: Implement automatic token refresh if supported by the API
2. **Multiple Endpoints**: Support for different regional endpoints
3. **Token Validation**: Pre-validate tokens before making API calls
4. **Caching**: Implement response caching for better performance

## Testing

The implementation has been tested with:

- Valid personal access tokens
- Invalid/expired tokens
- Network connectivity issues
- Configuration file validation
- Fallback to AccessKey authentication
- Panic prevention when using token authentication
- Dual authentication mode switching

### API Endpoint Testing Results

✅ **ListPipelines API** - Successfully working
- Endpoint: `GET /oapi/v1/flow/organizations/{organizationId}/pipelines`
- Authentication: `X-Yunxiao-Token` header
- Response: JSON array of pipeline objects
- Status: Fully functional, returns 93 pipelines in test environment

❌ **ListPipelineGroups API** - Needs correct endpoint
- Current endpoint: `/oapi/v1/organizations/{organizationId}/projects` (returns HTML)
- Status: Endpoint path needs to be updated with correct API documentation

✅ **Authentication Method** - Working correctly
- Header: `X-Yunxiao-Token: {personal_access_token}`
- Domain: `openapi-rdc.aliyuncs.com`
- Status: Successfully authenticating API requests

## Bug Fixes

### Fixed: Panic when using personal access token authentication

**Issue**: The application would panic with a nil pointer dereference when using personal access token authentication because some methods still tried to use the `sdkClient` which is `nil` for token-based authentication.

**Solution**: Added dual authentication support to all API methods:
- `ListPipelines` → `listPipelinesWithToken`
- `ListPipelineGroups` → `listPipelineGroupsWithToken`
- `RunPipeline` → `runPipelineWithToken`
- `GetPipelineRun` → `getPipelineRunWithToken`

Each method now checks the `useToken` flag and routes to the appropriate implementation.

## References

- [Aliyun DevOps Personal Access Token Documentation](https://help.aliyun.com/zh/yunxiao/developer-reference/obtain-personal-access-token)
- [Aliyun DevOps Service Access Points](https://help.aliyun.com/zh/yunxiao/developer-reference/service-access-point-domain)
- [Project Configuration Guide](.cursor/rules/configuration-guide.mdc) 