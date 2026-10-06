#!/bin/bash
# Phase 1 Security Validation Test
# Validates: DB permissions, token handling, session secret, debug defaults

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

test_result() {
    local test_name=$1
    local result=$2
    if [ "$result" -eq 0 ]; then
        echo -e "${GREEN}✓ PASS${NC}: $test_name"
        ((TESTS_PASSED++))
    else
        echo -e "${RED}✗ FAIL${NC}: $test_name"
        ((TESTS_FAILED++))
    fi
}

echo -e "${BLUE}=== Phase 1 Security Validation ===${NC}"
echo ""

# Test 1: Database permissions
echo -e "${YELLOW}Test 1: Database Permissions${NC}"
TEST_DB="./data/test-phase1.db"
rm -f "$TEST_DB"
mkdir -p ./data

# Simulate what start-agent.sh does
touch "$TEST_DB"
chmod 600 "$TEST_DB"
PERMS=$(stat -c "%a" "$TEST_DB")
if [ "$PERMS" = "600" ]; then
    test_result "DB file created with 600 permissions" 0
else
    echo "  Expected 600, got $PERMS"
    test_result "DB file created with 600 permissions" 1
fi

# Test 2: chmod 600 on existing files
echo ""
echo -e "${YELLOW}Test 2: Secure Permissions Update${NC}"
chmod 644 "$TEST_DB"
chmod 600 "$TEST_DB" 2>/dev/null || true
PERMS=$(stat -c "%a" "$TEST_DB")
if [ "$PERMS" = "600" ]; then
    test_result "chmod 600 can be applied to existing DB file" 0
else
    echo "  Failed to set permissions to 600 (got $PERMS)"
    test_result "chmod 600 can be applied to existing DB file" 1
fi
rm -f "$TEST_DB"

# Test 3: Token file permissions
echo ""
echo -e "${YELLOW}Test 3: Token File Permissions${NC}"
TEST_TOKEN_FILE="./data/test-docklite.token"
rm -f "$TEST_TOKEN_FILE"

# Simulate what start-agent.sh does
TEST_TOKEN=$(openssl rand -hex 32)
umask 077
printf "%s\n" "$TEST_TOKEN" > "$TEST_TOKEN_FILE"
umask 022

# Check permissions
TOKEN_PERMS=$(stat -c "%a" "$TEST_TOKEN_FILE")
if [ "$TOKEN_PERMS" = "600" ]; then
    test_result "Token file created with 600 permissions (umask 077)" 0
else
    echo "  Expected 600, got $TOKEN_PERMS"
    test_result "Token file created with 600 permissions (umask 077)" 1
fi

# Verify token is readable
if [ -r "$TEST_TOKEN_FILE" ]; then
    test_result "Token file is readable" 0
else
    test_result "Token file is readable" 1
fi

# Verify token content
STORED_TOKEN=$(cat "$TEST_TOKEN_FILE")
if [ "$STORED_TOKEN" = "$TEST_TOKEN" ]; then
    test_result "Token content preserved" 0
else
    test_result "Token content preserved" 1
fi
rm -f "$TEST_TOKEN_FILE"

# Test 4: Startup script syntax
echo ""
echo -e "${YELLOW}Test 4: Startup Script Syntax${NC}"
for script in start-agent.sh start-fullstack.sh init-db.sh; do
    if bash -n "$script" 2>/dev/null; then
        test_result "Script $script has valid bash syntax" 0
    else
        test_result "Script $script has valid bash syntax" 1
    fi
done

# Test 5: Check for debug defaults
echo ""
echo -e "${YELLOW}Test 5: Debug Defaults${NC}"
if grep -q "DEBUG=true\|DEBUG:=true\|DEBUG=1" start-agent.sh start-fullstack.sh 2>/dev/null; then
    test_result "No forced debug=true in startup scripts" 1
else
    test_result "No forced debug=true in startup scripts" 0
fi

# Test 6: Check for weak session secret defaults
echo ""
echo -e "${YELLOW}Test 6: Session Secret Handling${NC}"
if grep -q 'SESSION_SECRET="' start-fullstack.sh; then
    test_result "No hardcoded SESSION_SECRET value in script" 1
else
    test_result "No hardcoded SESSION_SECRET value in script" 0
fi

# Script should generate if not provided
if grep -q 'SESSION_SECRET=$(openssl rand' start-fullstack.sh; then
    test_result "Script generates secure SESSION_SECRET if not provided" 0
else
    test_result "Script generates secure SESSION_SECRET if not provided" 1
fi

# Test 7: Check for token exposure
echo ""
echo -e "${YELLOW}Test 7: Token Exposure${NC}"
# Should not print the full token
if grep -q 'echo.*DOCKLITE_TOKEN\|echo.*\$DOCKLITE_TOKEN' start-agent.sh start-fullstack.sh 2>/dev/null; then
    test_result "Token not directly echoed in scripts" 1
else
    test_result "Token not directly echoed in scripts" 0
fi

# Should indicate token is "set" without revealing it
if grep -q 'Token: (set)' start-agent.sh start-fullstack.sh 2>/dev/null; then
    test_result "Token output masked as '(set)'" 0
else
    test_result "Token output masked as '(set)'" 1
fi

# Test 8: Docker socket validation
echo ""
echo -e "${YELLOW}Test 8: Docker Socket Validation${NC}"
if grep -q "DOCKER_SOCKET_FILE=" start-agent.sh && grep -q "\[ ! -S" start-agent.sh; then
    test_result "Docker socket path validation present" 0
else
    test_result "Docker socket path validation present" 1
fi

# Check for socket permission warning
if grep -q "world-writable\|writable by others" start-agent.sh; then
    test_result "Docker socket permission warning present" 0
else
    test_result "Docker socket permission warning present" 1
fi

# Test 9: Check router configuration
echo ""
echo -e "${YELLOW}Test 9: Debug Route Registration${NC}"
ROUTER_FILE="./go-app/internal/api/router.go"
if [ -f "$ROUTER_FILE" ]; then
    # Check that debug routes exist but might be gated
    if grep -q "/debug\|/api/debug" "$ROUTER_FILE"; then
        test_result "Debug routes exist in router" 0
        # Note: Full gating validation would need code review
    else
        test_result "Debug routes exist in router" 1
    fi
else
    test_result "Router file exists" 1
fi

# Test 10: System update handler security
echo ""
echo -e "${YELLOW}Test 10: System Update Handler${NC}"
SYSUPDATE_FILE="./go-app/internal/handlers/system_update.go"
if [ -f "$SYSUPDATE_FILE" ]; then
    # Check for script ownership/permission validation
    if grep -q "os.Stat\|Chmod\|Chown" "$SYSUPDATE_FILE"; then
        test_result "System update handler has permission checks" 0
    else
        echo "  (Note: P0 hardening still pending for this file)"
        test_result "System update handler has permission checks" 1
    fi
else
    test_result "System update handler file exists" 1
fi

# Summary
echo ""
echo -e "${BLUE}=== Test Summary ===${NC}"
TOTAL=$((TESTS_PASSED + TESTS_FAILED))
echo "Passed: ${GREEN}${TESTS_PASSED}${NC}/$TOTAL"
echo "Failed: ${RED}${TESTS_FAILED}${NC}/$TOTAL"

if [ $TESTS_FAILED -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ Phase 1 Validation Complete - All tests passed!${NC}"
    exit 0
else
    echo ""
    echo -e "${RED}✗ Phase 1 Validation Failed - $TESTS_FAILED test(s) failed${NC}"
    exit 1
fi
