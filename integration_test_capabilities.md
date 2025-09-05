# MCP Capabilities API Integration Test Guide

## Test Environment Setup

### Prerequisites
1. Running apiserver instance
2. At least one configured MCP server
3. Valid JWT token for authentication
4. Test tenant with appropriate permissions

### Test Data Setup

Create a test MCP configuration:

```yaml
# test-mcp-config.yaml
name: test-server
tenant: test-tenant
mcpServers:
  - name: mock-mcp-server
    type: stdio
    command: python
    args: ["-m", "http.server", "8000"]
    policy: onDemand
    preinstalled: false
tools:
  - name: test-tool
    description: A test tool for integration testing
    method: GET
    endpoint: http://localhost:8000/api/test
    requestBody: ""
    responseBody: ""
prompts:
  - name: test-prompt
    description: A test prompt for integration testing
    arguments:
      - name: input
        description: Test input parameter
        required: true
routers:
  - server: mock-mcp-server
    prefix: /test-tenant/mock
```

## Manual Test Cases

### Test Case 1: Successful Capabilities Retrieval

**Request:**
```bash
curl -X GET \
  "http://localhost:5234/api/mcp/capabilities/test-tenant/test-server" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Accept: application/json" \
  -v
```

**Expected Response:**
- Status Code: 200 OK
- Content-Type: application/json
- Body structure matches API documentation
- Contains tools, prompts, resources, resourceTemplates arrays
- lastSynced timestamp is present

### Test Case 2: Missing Authentication

**Request:**
```bash
curl -X GET \
  "http://localhost:5234/api/mcp/capabilities/test-tenant/test-server" \
  -H "Accept: application/json" \
  -v
```

**Expected Response:**
- Status Code: 401 Unauthorized
- Error code: "ErrorUnauthorized"

### Test Case 3: Missing Tenant Parameter

**Request:**
```bash
curl -X GET \
  "http://localhost:5234/api/mcp/capabilities//test-server" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Accept: application/json" \
  -v
```

**Expected Response:**
- Status Code: 400 Bad Request
- Error code: "ErrorTenantRequired"

### Test Case 4: Missing Server Name Parameter

**Request:**
```bash
curl -X GET \
  "http://localhost:5234/api/mcp/capabilities/test-tenant/" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Accept: application/json" \
  -v
```

**Expected Response:**
- Status Code: 400 Bad Request  
- Error code: "ErrorMCPServerNameRequired"

### Test Case 5: Non-existent MCP Server

**Request:**
```bash
curl -X GET \
  "http://localhost:5234/api/mcp/capabilities/test-tenant/nonexistent-server" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Accept: application/json" \
  -v
```

**Expected Response:**
- Status Code: 404 Not Found
- Error code: "ErrorMCPServerNotFound"

### Test Case 6: Insufficient Permissions

**Request:** (using a JWT token for a user without access to test-tenant)
```bash
curl -X GET \
  "http://localhost:5234/api/mcp/capabilities/test-tenant/test-server" \
  -H "Authorization: Bearer $LIMITED_JWT_TOKEN" \
  -H "Accept: application/json" \
  -v
```

**Expected Response:**
- Status Code: 403 Forbidden
- Error code: "ErrorTenantPermissionError"

### Test Case 7: Cache Validation

**Request 1:**
```bash
time curl -X GET \
  "http://localhost:5234/api/mcp/capabilities/test-tenant/test-server" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Accept: application/json" \
  -s -o /dev/null -w "%{time_total}"
```

**Request 2 (immediate):**
```bash
time curl -X GET \
  "http://localhost:5234/api/mcp/capabilities/test-tenant/test-server" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Accept: application/json" \
  -s -o /dev/null -w "%{time_total}"
```

**Expected:**
- Second request should be significantly faster due to caching
- Both requests should return identical data

## Automated Test Script

```bash
#!/bin/bash

# MCP Capabilities API Integration Test Script

BASE_URL="http://localhost:5234"
TENANT="test-tenant"
SERVER="test-server"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results tracking
PASSED=0
FAILED=0

# Function to run test
run_test() {
    local test_name="$1"
    local expected_status="$2"
    local curl_cmd="$3"
    
    echo -e "${YELLOW}Running: $test_name${NC}"
    
    response=$(eval "$curl_cmd")
    status_code=$(echo "$response" | tail -n1)
    response_body=$(echo "$response" | head -n -1)
    
    if [ "$status_code" -eq "$expected_status" ]; then
        echo -e "${GREEN}✓ PASSED${NC}: $test_name"
        ((PASSED++))
    else
        echo -e "${RED}✗ FAILED${NC}: $test_name"
        echo "Expected: $expected_status, Got: $status_code"
        echo "Response: $response_body"
        ((FAILED++))
    fi
    echo ""
}

# Check if JWT token is set
if [ -z "$JWT_TOKEN" ]; then
    echo -e "${RED}Error: JWT_TOKEN environment variable not set${NC}"
    echo "Please set it with: export JWT_TOKEN='your_token_here'"
    exit 1
fi

echo "Starting MCP Capabilities API Integration Tests..."
echo "Base URL: $BASE_URL"
echo "Test Tenant: $TENANT"
echo "Test Server: $SERVER"
echo ""

# Test 1: Successful capabilities retrieval
run_test "Successful capabilities retrieval" 200 \
"curl -s -w '%{http_code}' '$BASE_URL/api/mcp/capabilities/$TENANT/$SERVER' \
-H 'Authorization: Bearer $JWT_TOKEN' \
-H 'Accept: application/json'"

# Test 2: Missing authentication
run_test "Missing authentication" 401 \
"curl -s -w '%{http_code}' '$BASE_URL/api/mcp/capabilities/$TENANT/$SERVER' \
-H 'Accept: application/json'"

# Test 3: Missing tenant parameter
run_test "Missing tenant parameter" 400 \
"curl -s -w '%{http_code}' '$BASE_URL/api/mcp/capabilities//$SERVER' \
-H 'Authorization: Bearer $JWT_TOKEN' \
-H 'Accept: application/json'"

# Test 4: Missing server name parameter
run_test "Missing server name parameter" 400 \
"curl -s -w '%{http_code}' '$BASE_URL/api/mcp/capabilities/$TENANT/' \
-H 'Authorization: Bearer $JWT_TOKEN' \
-H 'Accept: application/json'"

# Test 5: Non-existent MCP server
run_test "Non-existent MCP server" 404 \
"curl -s -w '%{http_code}' '$BASE_URL/api/mcp/capabilities/$TENANT/nonexistent-server' \
-H 'Authorization: Bearer $JWT_TOKEN' \
-H 'Accept: application/json'"

# Test 6: Cache performance test
echo -e "${YELLOW}Running: Cache performance test${NC}"
echo "First request (no cache):"
time1=$(curl -s -w '%{time_total}' -o /dev/null "$BASE_URL/api/mcp/capabilities/$TENANT/$SERVER" \
-H "Authorization: Bearer $JWT_TOKEN" \
-H "Accept: application/json")

echo "Second request (cached):"
time2=$(curl -s -w '%{time_total}' -o /dev/null "$BASE_URL/api/mcp/capabilities/$TENANT/$SERVER" \
-H "Authorization: Bearer $JWT_TOKEN" \
-H "Accept: application/json")

echo "First request time: ${time1}s"
echo "Second request time: ${time2}s"

# Simple cache validation (second request should be faster)
if (( $(echo "$time2 < $time1" | bc -l) )); then
    echo -e "${GREEN}✓ PASSED${NC}: Cache performance test"
    ((PASSED++))
else
    echo -e "${RED}✗ FAILED${NC}: Cache performance test"
    echo "Second request was not faster than first request"
    ((FAILED++))
fi
echo ""

# Summary
echo "===================="
echo "Test Summary:"
echo -e "${GREEN}Passed: $PASSED${NC}"
echo -e "${RED}Failed: $FAILED${NC}"
echo "Total: $((PASSED + FAILED))"
echo "===================="

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed.${NC}"
    exit 1
fi
```

## Usage Instructions

1. **Set up environment:**
   ```bash
   export JWT_TOKEN="your_jwt_token_here"
   export BASE_URL="http://localhost:5234"
   ```

2. **Make script executable:**
   ```bash
   chmod +x integration_test.sh
   ```

3. **Run tests:**
   ```bash
   ./integration_test.sh
   ```

## Expected Test Results

When all tests pass, you should see output similar to:

```
Starting MCP Capabilities API Integration Tests...
Base URL: http://localhost:5234
Test Tenant: test-tenant
Test Server: test-server

Running: Successful capabilities retrieval
✓ PASSED: Successful capabilities retrieval

Running: Missing authentication
✓ PASSED: Missing authentication

Running: Missing tenant parameter
✓ PASSED: Missing tenant parameter

Running: Missing server name parameter
✓ PASSED: Missing server name parameter

Running: Non-existent MCP server
✓ PASSED: Non-existent MCP server

Running: Cache performance test
First request time: 0.245s
Second request time: 0.012s
✓ PASSED: Cache performance test

====================
Test Summary:
Passed: 6
Failed: 0
Total: 6
====================
All tests passed!
```

## Troubleshooting

### Common Issues

1. **Connection refused:** Ensure apiserver is running on correct port
2. **Authentication failures:** Verify JWT token is valid and not expired
3. **Permission errors:** Ensure test user has access to test tenant
4. **MCP server not found:** Verify test MCP configuration exists
5. **Slow responses:** Check MCP server connectivity and performance

### Debug Commands

```bash
# Check if apiserver is running
curl -s http://localhost:5234/api/runtime-config

# Verify MCP server configuration exists
curl -s -H "Authorization: Bearer $JWT_TOKEN" \
  http://localhost:5234/api/mcp/configs

# Test JWT token validity
curl -s -H "Authorization: Bearer $JWT_TOKEN" \
  http://localhost:5234/api/auth/user/info
```