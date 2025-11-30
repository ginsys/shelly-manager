#!/bin/bash
# upgrade-go-version.sh - Upgrades Go version across all project files
#
# Updates:
# - .go-version
# - go.mod (go directive and toolchain)
# - mise.toml
#
# Usage: ./scripts/upgrade-go-version.sh 1.24.0

set -e

cd "$(dirname "$0")/.."

# Portable sed in-place replacement (works on both Linux and macOS)
# Uses sed -i.bak which works on both GNU and BSD sed, then removes backup
sed_inplace() {
    local file="$1"
    local expr="$2"
    sed -i.bak "$expr" "$file" && rm -f "${file}.bak"
}

NEW_VERSION="$1"

if [[ -z "$NEW_VERSION" ]]; then
    echo "Usage: $0 <version>"
    echo "Example: $0 1.24.0"
    echo ""
    if [[ -f .go-version ]]; then
        CURRENT=$(cat .go-version | tr -d '[:space:]')
        echo "Current version: ${CURRENT}"
    fi
    exit 1
fi

# Validate version format (should be like 1.23.0 or 1.24)
if ! [[ "$NEW_VERSION" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]]; then
    echo "ERROR: Invalid version format: ${NEW_VERSION}"
    echo "Expected format: X.Y or X.Y.Z (e.g., 1.24 or 1.24.0)"
    exit 1
fi

MAJOR_MINOR="${NEW_VERSION%.*}"
if [[ "$NEW_VERSION" == "$MAJOR_MINOR" ]]; then
    # No patch version provided, keep as is for .go-version
    MAJOR_MINOR="${NEW_VERSION}"
fi

echo "Upgrading Go version to ${NEW_VERSION}..."
echo ""

# Update .go-version
echo "${NEW_VERSION}" > .go-version
echo "  Updated .go-version"

# Update go.mod
if [[ -f go.mod ]]; then
    # Update go directive (use major.minor only)
    sed_inplace go.mod "s/^go [0-9.]\+$/go ${MAJOR_MINOR}/"

    # Update or add toolchain directive
    if grep -q "^toolchain go" go.mod; then
        sed_inplace go.mod "s/^toolchain go[0-9.]\+$/toolchain go${NEW_VERSION}/"
    else
        # Add toolchain after go directive
        sed_inplace go.mod "/^go ${MAJOR_MINOR}$/a toolchain go${NEW_VERSION}"
    fi
    echo "  Updated go.mod"
fi

# Update mise.toml
if [[ -f mise.toml ]]; then
    if grep -q "^go = " mise.toml; then
        sed_inplace mise.toml "s/^go = \"[^\"]*\"/go = \"${NEW_VERSION}\"/"
        echo "  Updated mise.toml"
    else
        echo "  mise.toml: no go version to update (skipped)"
    fi
fi

# Run go mod tidy to ensure consistency
echo ""
echo "Running go mod tidy..."
go mod tidy

echo ""
echo "Done! Go version updated to ${NEW_VERSION}"
echo ""
echo "Next steps:"
echo "  1. Run: ./scripts/check-go-version.sh"
echo "  2. Run: make test"
echo "  3. Commit the changes"
