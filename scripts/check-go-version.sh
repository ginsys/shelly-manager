#!/bin/bash
# check-go-version.sh - Validates Go version consistency across all project files
#
# This script checks that the Go version is consistent across:
# - .go-version (source of truth)
# - go.mod
# - mise.toml
# - GitHub Actions workflows (should use go-version-file, not hardcoded)
#
# It also validates that dependencies don't require a newer Go version.

set -e

cd "$(dirname "$0")/.."

ERRORS=0
WARNINGS=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Read expected version from .go-version
if [[ ! -f .go-version ]]; then
    echo -e "${RED}ERROR: .go-version file not found${NC}"
    exit 1
fi

EXPECTED=$(cat .go-version | tr -d '[:space:]')
EXPECTED_MAJOR_MINOR="${EXPECTED%.*}"  # e.g., "1.23" from "1.23.0"

echo "Checking Go version consistency..."
echo "Expected version: ${EXPECTED} (${EXPECTED_MAJOR_MINOR}.x)"
echo ""

# Check go.mod
if [[ -f go.mod ]]; then
    GOMOD_VERSION=$(grep "^go " go.mod | awk '{print $2}')
    GOMOD_TOOLCHAIN=$(grep "^toolchain go" go.mod | sed 's/toolchain go//')
    GOMOD_MAJOR_MINOR="${GOMOD_VERSION%.*}"

    if [[ -z "$GOMOD_VERSION" ]]; then
        echo -e "${RED}  go.mod: missing go directive${NC}"
        ERRORS=$((ERRORS+1))
    elif [[ "$GOMOD_MAJOR_MINOR" == "$EXPECTED_MAJOR_MINOR" ]]; then
        # Major.minor matches, check toolchain if present
        if [[ -n "$GOMOD_TOOLCHAIN" ]]; then
            if [[ "$GOMOD_TOOLCHAIN" == "$EXPECTED" ]]; then
                echo -e "${GREEN}  go.mod: go ${GOMOD_VERSION}, toolchain go${GOMOD_TOOLCHAIN}${NC}"
            else
                echo -e "${YELLOW}  go.mod: go ${GOMOD_VERSION}, toolchain go${GOMOD_TOOLCHAIN} (toolchain differs from ${EXPECTED})${NC}"
                WARNINGS=$((WARNINGS+1))
            fi
        else
            echo -e "${GREEN}  go.mod: ${GOMOD_VERSION}${NC}"
        fi
    else
        echo -e "${RED}  go.mod: ${GOMOD_VERSION} (expected ${EXPECTED_MAJOR_MINOR}.x)${NC}"
        ERRORS=$((ERRORS+1))
    fi
else
    echo -e "${RED}  go.mod: file not found${NC}"
    ERRORS=$((ERRORS+1))
fi

# Check mise.toml
if [[ -f mise.toml ]]; then
    MISE_VERSION=$(grep "^go = " mise.toml 2>/dev/null | sed 's/go = "\([^"]*\)"/\1/')
    if [[ -z "$MISE_VERSION" ]]; then
        echo -e "${YELLOW}  mise.toml: no go version defined (warning)${NC}"
        WARNINGS=$((WARNINGS+1))
    elif [[ "$MISE_VERSION" == "$EXPECTED" || "$MISE_VERSION" == "$EXPECTED_MAJOR_MINOR" ]]; then
        echo -e "${GREEN}  mise.toml: ${MISE_VERSION}${NC}"
    else
        echo -e "${RED}  mise.toml: ${MISE_VERSION} (expected ${EXPECTED})${NC}"
        ERRORS=$((ERRORS+1))
    fi
fi

# Check GitHub Actions workflows
echo ""
echo "Checking GitHub Actions workflows..."
WORKFLOW_ERRORS=0

for workflow in .github/workflows/*.yml; do
    if [[ ! -f "$workflow" ]]; then
        continue
    fi

    filename=$(basename "$workflow")

    # Check for hardcoded go-version (bad) vs go-version-file (good)
    if grep -q "go-version:" "$workflow" && ! grep -q "go-version-file:" "$workflow"; then
        # Has go-version but not go-version-file - check if it's using a variable
        if grep -q 'go-version:.*\${{' "$workflow"; then
            # Using a variable is acceptable
            VAR_VALUE=$(grep "GO_VERSION:" "$workflow" | head -1 | awk -F"'" '{print $2}')
            if [[ -n "$VAR_VALUE" && "$VAR_VALUE" != "$EXPECTED_MAJOR_MINOR" && "$VAR_VALUE" != "$EXPECTED" ]]; then
                echo -e "${RED}  ${filename}: GO_VERSION=${VAR_VALUE} (expected ${EXPECTED_MAJOR_MINOR})${NC}"
                WORKFLOW_ERRORS=$((WORKFLOW_ERRORS+1))
            else
                echo -e "${YELLOW}  ${filename}: uses variable GO_VERSION (consider using go-version-file)${NC}"
                WARNINGS=$((WARNINGS+1))
            fi
        else
            echo -e "${RED}  ${filename}: has hardcoded go-version (should use go-version-file)${NC}"
            WORKFLOW_ERRORS=$((WORKFLOW_ERRORS+1))
        fi
    elif grep -q "go-version-file:" "$workflow"; then
        echo -e "${GREEN}  ${filename}: uses go-version-file${NC}"
    elif grep -q "setup-go" "$workflow"; then
        echo -e "${YELLOW}  ${filename}: uses setup-go but couldn't determine version method${NC}"
        WARNINGS=$((WARNINGS+1))
    fi
done

ERRORS=$((ERRORS+WORKFLOW_ERRORS))

# Check that dependencies are compatible with current Go version
echo ""
echo "Checking dependency compatibility..."

# Try to download dependencies and catch version errors
DOWNLOAD_OUTPUT=$(go mod download 2>&1) || true

if echo "$DOWNLOAD_OUTPUT" | grep -q "requires go >="; then
    REQUIRED=$(echo "$DOWNLOAD_OUTPUT" | grep -o "requires go >= [0-9.]*" | head -1 | awk '{print $NF}')
    echo -e "${RED}  Dependencies require Go >= ${REQUIRED} (current: ${EXPECTED})${NC}"
    ERRORS=$((ERRORS+1))
elif echo "$DOWNLOAD_OUTPUT" | grep -q "go.mod requires go"; then
    echo -e "${RED}  Dependency version conflict detected${NC}"
    echo "  Output: $DOWNLOAD_OUTPUT"
    ERRORS=$((ERRORS+1))
else
    echo -e "${GREEN}  All dependencies compatible with Go ${EXPECTED}${NC}"
fi

# Summary
echo ""
echo "----------------------------------------"
if [[ $ERRORS -eq 0 && $WARNINGS -eq 0 ]]; then
    echo -e "${GREEN}All checks passed!${NC}"
    exit 0
elif [[ $ERRORS -eq 0 ]]; then
    echo -e "${YELLOW}Passed with ${WARNINGS} warning(s)${NC}"
    exit 0
else
    echo -e "${RED}Failed with ${ERRORS} error(s) and ${WARNINGS} warning(s)${NC}"
    echo ""
    echo "To fix version mismatches, run:"
    echo "  ./scripts/upgrade-go-version.sh ${EXPECTED}"
    exit 1
fi
