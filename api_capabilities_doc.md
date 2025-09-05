# MCP Capabilities API Documentation

## Overview

The MCP Capabilities API endpoint allows clients to retrieve capability information from MCP (Model Context Protocol) services, including available tools, prompts, resources, and resource templates.

## Endpoint

```
GET /api/mcp/capabilities/{tenant}/{name}
```

## Authentication

This endpoint requires JWT authentication. Include a valid JWT token in the Authorization header:

```
Authorization: Bearer <jwt_token>
```

## Parameters

### Path Parameters

| Parameter | Type   | Required | Description                              |
|-----------|--------|----------|------------------------------------------|
| tenant    | string | Yes      | The tenant name that owns the MCP server |
| name      | string | Yes      | The MCP server configuration name        |

## Permissions

- Users must have access to the specified tenant
- Admin users can access all tenants
- Regular users can only access tenants they are assigned to

## Response Format

### Success Response (200 OK)

```json
{
  "success": true,
  "message": "MCP capabilities retrieved successfully",
  "data": {
    "tools": [
      {
        "name": "string",
        "description": "string",
        "inputSchema": {
          "type": "object",
          "properties": {},
          "required": [],
          "title": "string"
        },
        "annotations": {
          "title": "string",
          "destructiveHint": false,
          "idempotentHint": true,
          "readOnlyHint": false,
          "openWorldHint": false
        },
        "enabled": true,
        "lastSynced": "2023-12-07T10:30:00Z"
      }
    ],
    "prompts": [
      {
        "name": "string",
        "description": "string",
        "arguments": [
          {
            "name": "string",
            "description": "string",
            "required": true
          }
        ],
        "promptResponse": [
          {
            "role": "user",
            "content": {
              "type": "text",
              "text": "string"
            }
          }
        ],
        "lastSynced": "2023-12-07T10:30:00Z"
      }
    ],
    "resources": [
      {
        "uri": "string",
        "name": "string",
        "description": "string",
        "mimeType": "string",
        "lastSynced": "2023-12-07T10:30:00Z"
      }
    ],
    "resourceTemplates": [
      {
        "uriTemplate": "string",
        "name": "string",
        "description": "string",
        "mimeType": "string",
        "parameters": [
          {
            "name": "string",
            "description": "string",
            "required": true,
            "type": "string"
          }
        ],
        "lastSynced": "2023-12-07T10:30:00Z"
      }
    ],
    "lastSynced": "2023-12-07T10:30:00Z",
    "serverInfo": {
      "name": "string",
      "version": "string"
    }
  }
}
```

### Error Responses

#### 400 Bad Request
Missing required parameters:

```json
{
  "success": false,
  "error": {
    "code": "ErrorTenantRequired",
    "message": "Tenant name is required"
  }
}
```

#### 401 Unauthorized
Missing or invalid authentication:

```json
{
  "success": false,
  "error": {
    "code": "ErrorUnauthorized", 
    "message": "Unauthorized access"
  }
}
```

#### 403 Forbidden
Insufficient permissions:

```json
{
  "success": false,
  "error": {
    "code": "ErrorTenantPermissionError",
    "message": "You do not have permission to access this tenant"
  }
}
```

#### 404 Not Found
MCP server not found:

```json
{
  "success": false,
  "error": {
    "code": "ErrorMCPServerNotFound",
    "message": "MCP server not found"
  }
}
```

#### 500 Internal Server Error
Server error during capability fetching:

```json
{
  "success": false,
  "error": {
    "code": "ErrorInternalServer",
    "message": "Failed to fetch capabilities: <error details>"
  }
}
```

## Implementation Details

### Caching
- Capabilities are cached for 5 minutes to improve performance
- Cache is automatically invalidated when MCP server configurations are updated, created, or deleted
- Cache key format: `{tenant}:{name}`

### Concurrent Fetching
- The API fetches tools, prompts, resources, and resource templates concurrently from all MCP servers in the configuration
- Partial failures are handled gracefully - if some capabilities can be fetched, they are returned with warnings logged

### Transport Support
- Supports all MCP transport types: SSE, stdio, and streamable-http
- Automatically starts and stops transports as needed
- Reuses existing transports when possible for efficiency

## Usage Examples

### cURL Example

```bash
# Get capabilities for a specific MCP server
curl -X GET \
  "http://localhost:5234/api/mcp/capabilities/my-tenant/my-server" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -H "Accept: application/json"
```

### JavaScript Example

```javascript
async function getMCPCapabilities(tenant, serverName, token) {
  try {
    const response = await fetch(`/api/mcp/capabilities/${tenant}/${serverName}`, {
      method: 'GET',
      headers: {
        'Authorization': `Bearer ${token}`,
        'Accept': 'application/json',
      },
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();
    return data.data; // Extract capabilities data
  } catch (error) {
    console.error('Failed to fetch MCP capabilities:', error);
    throw error;
  }
}

// Usage
const capabilities = await getMCPCapabilities('my-tenant', 'my-server', 'jwt_token');
console.log(`Found ${capabilities.tools.length} tools`);
console.log(`Found ${capabilities.prompts.length} prompts`);
```

### Python Example

```python
import requests
import json

def get_mcp_capabilities(base_url, tenant, server_name, token):
    """
    Get MCP capabilities for a specific server
    """
    url = f"{base_url}/api/mcp/capabilities/{tenant}/{server_name}"
    headers = {
        'Authorization': f'Bearer {token}',
        'Accept': 'application/json',
    }
    
    try:
        response = requests.get(url, headers=headers)
        response.raise_for_status()
        
        data = response.json()
        return data['data']
    except requests.exceptions.RequestException as e:
        print(f"Error fetching capabilities: {e}")
        raise

# Usage
capabilities = get_mcp_capabilities(
    'http://localhost:5234',
    'my-tenant', 
    'my-server',
    'jwt_token'
)

print(f"Found {len(capabilities['tools'])} tools")
print(f"Found {len(capabilities['prompts'])} prompts")
```

## Rate Limiting

The API implements intelligent caching to reduce load on MCP servers:
- Results are cached for 5 minutes per server
- Subsequent requests within the cache period return cached data immediately
- Cache is invalidated when server configurations change

## Best Practices

1. **Authentication**: Always include valid JWT tokens in requests
2. **Error Handling**: Implement proper error handling for all possible HTTP status codes
3. **Caching**: Leverage the built-in caching by not making excessive requests
4. **Permissions**: Ensure users have appropriate tenant access before making requests
5. **Monitoring**: Monitor response times and error rates for operational insights

## Related Endpoints

- `GET /api/mcp/configs` - List all MCP server configurations
- `GET /api/mcp/configs/{tenant}/{name}` - Get specific MCP server configuration  
- `POST /api/mcp/configs` - Create new MCP server configuration
- `PUT /api/mcp/configs` - Update MCP server configuration
- `DELETE /api/mcp/configs/{tenant}/{name}` - Delete MCP server configuration