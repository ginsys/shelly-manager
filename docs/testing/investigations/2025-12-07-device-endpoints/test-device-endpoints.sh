#!/bin/bash
# TEMPORARY INVESTIGATION SCRIPT
# Tests all device API endpoints to see which work and which fail
# Delete after investigation

API_BASE="http://localhost:8080"
DEVICE_ID=""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "Device Endpoints Investigation"
echo "========================================"
echo

# Get a device ID to test with
echo "Fetching device list..."
DEVICES_RESP=$(curl -s "$API_BASE/api/v1/devices")
DEVICE_ID=$(echo "$DEVICES_RESP" | jq -r '.data.devices[0].id // empty')

if [ -z "$DEVICE_ID" ]; then
  echo "${RED}✗ No devices found. Creating test device...${NC}"
  CREATE_RESP=$(curl -s -X POST "$API_BASE/api/v1/devices" \
    -H "Content-Type: application/json" \
    -d '{"name":"Investigation Device","ip_address":"172.31.103.199","device_type":"shelly1","enabled":false}')

  DEVICE_ID=$(echo "$CREATE_RESP" | jq -r '.data.id // empty')

  if [ -z "$DEVICE_ID" ]; then
    echo "${RED}✗ Failed to create test device${NC}"
    echo "$CREATE_RESP" | jq '.'
    exit 1
  fi
  echo "${GREEN}✓ Created test device ID: $DEVICE_ID${NC}"
else
  echo "${GREEN}✓ Using existing device ID: $DEVICE_ID${NC}"
fi

echo
echo "Testing 27 device endpoints..."
echo

# Helper function to test an endpoint
test_endpoint() {
  local method=$1
  local path=$2
  local data=$3
  local desc=$4
  local accept_codes=$5

  # Replace {id} with actual device ID
  path=$(echo "$path" | sed "s/{id}/$DEVICE_ID/g")

  if [ "$method" = "GET" ]; then
    RESP=$(curl -s -w "\n%{http_code}" "$API_BASE$path")
  elif [ "$method" = "POST" ]; then
    RESP=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE$path" \
      -H "Content-Type: application/json" \
      -d "$data")
  elif [ "$method" = "PUT" ]; then
    RESP=$(curl -s -w "\n%{http_code}" -X PUT "$API_BASE$path" \
      -H "Content-Type: application/json" \
      -d "$data")
  elif [ "$method" = "DELETE" ]; then
    RESP=$(curl -s -w "\n%{http_code}" -X DELETE "$API_BASE$path")
  fi

  # Extract status code (last line)
  STATUS=$(echo "$RESP" | tail -1)
  # Extract body (everything except last line)
  BODY=$(echo "$RESP" | head -n -1)

  # Check if status is acceptable
  if echo "$accept_codes" | grep -q "$STATUS"; then
    echo -e "${GREEN}✓${NC} $method $(printf '%-50s' "$path") → $STATUS"
  else
    echo -e "${RED}✗${NC} $method $(printf '%-50s' "$path") → $STATUS"
  fi

  # Show error if not 2xx
  if [ "$STATUS" -ge 400 ]; then
    ERROR=$(echo "$BODY" | jq -r '.error // .message // empty' 2>/dev/null)
    if [ -n "$ERROR" ]; then
      echo "   Error: $ERROR"
    fi
  fi
}

# Core CRUD
echo "## Core CRUD"
test_endpoint "GET" "/api/v1/devices" "" "List devices" "200"
test_endpoint "POST" "/api/v1/devices" '{"name":"Temp","ip_address":"172.31.103.198","device_type":"shelly1"}' "Create device" "200,201,409"
test_endpoint "GET" "/api/v1/devices/{id}" "" "Get device" "200"
test_endpoint "PUT" "/api/v1/devices/{id}" '{"name":"Updated"}' "Update device" "200"
echo

# Control & Status
echo "## Control & Status"
test_endpoint "POST" "/api/v1/devices/{id}/control" '{"action":"status"}' "Control device" "200,408,500"
test_endpoint "GET" "/api/v1/devices/{id}/status" "" "Get status" "200,408,500"
test_endpoint "GET" "/api/v1/devices/{id}/energy" "" "Get energy" "200,408,500"
echo

# Configuration
echo "## Configuration"
test_endpoint "GET" "/api/v1/devices/{id}/config" "" "Get stored config" "200,404"
test_endpoint "PUT" "/api/v1/devices/{id}/config" '{"config":{"test":true}}' "Update config" "200,400,404"
test_endpoint "GET" "/api/v1/devices/{id}/config/current" "" "Get live config" "200,408,500"
test_endpoint "GET" "/api/v1/devices/{id}/config/current/normalized" "" "Get normalized live config" "200,408,500"
test_endpoint "GET" "/api/v1/devices/{id}/config/typed/normalized" "" "Get typed normalized config" "200,404,408,500"
test_endpoint "POST" "/api/v1/devices/{id}/config/import" '{}' "Import config" "200,202,408,500"
test_endpoint "GET" "/api/v1/devices/{id}/config/status" "" "Get import status" "200,404"
test_endpoint "POST" "/api/v1/devices/{id}/config/export" '{}' "Export config" "200,202,404,408,500"
test_endpoint "GET" "/api/v1/devices/{id}/config/drift" "" "Detect drift" "200,404,500"
test_endpoint "POST" "/api/v1/devices/{id}/config/apply-template" '{"template_id":1}' "Apply template" "200,400,404"
test_endpoint "GET" "/api/v1/devices/{id}/config/history" "" "Get config history" "200,404"
test_endpoint "GET" "/api/v1/devices/{id}/config/typed" "" "Get typed config" "200,404"
test_endpoint "PUT" "/api/v1/devices/{id}/config/typed" '{"config":{"test":true}}' "Update typed config" "200,400,404"
echo

# Capability-Specific Config
echo "## Capability-Specific Config"
test_endpoint "PUT" "/api/v1/devices/{id}/config/relay" '{"relay":{"enabled":true}}' "Update relay config" "200,400,404"
test_endpoint "PUT" "/api/v1/devices/{id}/config/dimming" '{"dimming":{"enabled":false}}' "Update dimming config" "200,400,404"
test_endpoint "PUT" "/api/v1/devices/{id}/config/roller" '{"roller":{"enabled":false}}' "Update roller config" "200,400,404"
test_endpoint "PUT" "/api/v1/devices/{id}/config/power-metering" '{"power_metering":{"enabled":true}}' "Update power metering config" "200,400,404"
test_endpoint "PUT" "/api/v1/devices/{id}/config/auth" '{"auth":{"enabled":false}}' "Update auth config" "200,400,404"
echo

# Other
echo "## Other"
test_endpoint "GET" "/api/v1/devices/{id}/capabilities" "" "Get capabilities" "200,404"
echo

echo "========================================"
echo "Investigation Complete"
echo "========================================"
echo
echo "Summary:"
echo "- Green ✓ = Working as expected"
echo "- Red ✗ = Unexpected status code"
echo "- 408 = Timeout (expected for offline devices)"
echo "- 500 = Server error (investigate)"
echo "- 404 = Not found/not implemented"
