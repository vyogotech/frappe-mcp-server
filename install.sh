#!/bin/bash
#
# Frappe MCP Server Installation Script
# This script installs the Frappe MCP Server STDIO binary for use with
# MCP clients like Cursor IDE and Claude Desktop.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/vyogotech/frappe-mcp-server/main/install.sh | bash
#   OR
#   ./install.sh

set -e

# Configuration
VERSION="latest"
REPO="vyogotech/frappe-mcp-server"
BINARY_NAME="frappe-mcp-server-stdio"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
log_info() {
    echo -e "${BLUE}ℹ${NC} $1"
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

log_error() {
    echo -e "${RED}✗${NC} $1"
    exit 1
}

# Detect OS and Architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case "$os" in
        linux*)
            OS="linux"
            ;;
        darwin*)
            OS="darwin"
            ;;
        msys*|mingw*|cygwin*)
            OS="windows"
            ;;
        *)
            log_error "Unsupported operating system: $os"
            ;;
    esac
    
    case "$arch" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $arch"
            ;;
    esac
    
    log_info "Detected platform: ${OS}-${ARCH}"
}

# Get latest version from GitHub
get_latest_version() {
    if [ "$VERSION" = "latest" ]; then
        log_info "Fetching latest version..."
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
        if [ -z "$VERSION" ]; then
            log_error "Failed to fetch latest version"
        fi
        log_info "Latest version: v${VERSION}"
    fi
}

# Download binary
download_binary() {
    local filename="${BINARY_NAME}-${OS}-${ARCH}"
    local archive_name
    
    if [ "$OS" = "windows" ]; then
        filename="${filename}.exe"
        archive_name="${BINARY_NAME}-${OS}-${ARCH}.zip"
    else
        archive_name="${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
    fi
    
    local url="https://github.com/${REPO}/releases/download/v${VERSION}/${archive_name}"
    
    log_info "Downloading from: $url"
    
    local temp_dir=$(mktemp -d)
    cd "$temp_dir"
    
    if ! curl -fsSL -o "$archive_name" "$url"; then
        log_error "Failed to download binary. Please check if version v${VERSION} exists."
    fi
    
    log_success "Downloaded successfully"
    
    verify_checksum "$archive_name"
    
    # Extract archive
    log_info "Extracting archive..."
    if [ "$OS" = "windows" ]; then
        unzip -q "$archive_name"
    else
        tar -xzf "$archive_name"
    fi
    
    # Find the binary in extracted directory
    local extracted_dir="${BINARY_NAME}-${OS}-${ARCH}"
    if [ ! -f "${extracted_dir}/${filename}" ]; then
        log_error "Binary not found in archive"
    fi
    
    BINARY_PATH="${temp_dir}/${extracted_dir}/${filename}"
    CONFIG_TEMPLATE="${temp_dir}/${extracted_dir}/env.example"
    
    log_success "Extracted successfully"
}

# Verify the archive against the release's SHA256SUMS
verify_checksum() {
    local archive="$1"
    local sums_url="https://github.com/${REPO}/releases/download/v${VERSION}/SHA256SUMS"
    local checker
    
    if command -v sha256sum >/dev/null 2>&1; then
        checker="sha256sum"
    elif command -v shasum >/dev/null 2>&1; then
        checker="shasum -a 256"
    else
        log_error "Neither sha256sum nor shasum is available; cannot verify the download"
    fi
    
    log_info "Verifying checksum..."
    if ! curl -fsSL -o SHA256SUMS "$sums_url"; then
        log_error "Failed to download SHA256SUMS from $sums_url"
    fi
    
    grep " [ *]\{0,1\}${archive}\$" SHA256SUMS > SHA256SUMS.one || log_error "No checksum for ${archive} in SHA256SUMS"
    $checker -c SHA256SUMS.one >/dev/null 2>&1 || log_error "Checksum mismatch for ${archive}"
    
    log_success "Checksum verified"
}

# Install binary
install_binary() {
    log_info "Installing to ${INSTALL_DIR}..."
    
    # Create install directory if it doesn't exist
    mkdir -p "$INSTALL_DIR"
    
    # Copy binary
    cp "$BINARY_PATH" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    
    log_success "Installed binary to ${INSTALL_DIR}/${BINARY_NAME}"
    
    # Check if install directory is in PATH
    if [[ ":$PATH:" != *":${INSTALL_DIR}:"* ]]; then
        log_warn "Note: ${INSTALL_DIR} is not in your PATH"
        log_info "Add it to your PATH by adding this line to your ~/.bashrc or ~/.zshrc:"
        echo ""
        echo "    export PATH=\"\$PATH:${INSTALL_DIR}\""
        echo ""
    fi
}

# Create configuration directory
setup_config() {
    local config_dir="$HOME/.config/frappe-mcp-server"
    mkdir -p "$config_dir"
    
    if [ -f "$CONFIG_TEMPLATE" ] && [ ! -f "$config_dir/.env" ]; then
        cp "$CONFIG_TEMPLATE" "$config_dir/.env.example"
        log_success "Created config directory: $config_dir"
        log_info "Example configuration saved to: $config_dir/.env.example"
    fi
}

# Display next steps
show_next_steps() {
    echo ""
    echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
    echo -e "${GREEN}  Frappe MCP Server installed successfully! 🎉${NC}"
    echo -e "${GREEN}═══════════════════════════════════════════════════════${NC}"
    echo ""
    echo -e "${BLUE}Next Steps:${NC}"
    echo ""
    echo "1. Configure your Frappe/ERPNext credentials:"
    echo "   Create a config.yaml file with your settings"
    echo ""
    echo "2. Add to your MCP client configuration:"
    echo ""
    echo -e "${YELLOW}   For Cursor IDE (~/.cursor/mcp.json):${NC}"
    echo '   {'
    echo '     "mcpServers": {'
    echo '       "frappe": {'
    echo "         \"command\": \"${INSTALL_DIR}/${BINARY_NAME}\","
    echo '         "args": ["--config", "/path/to/your/config.yaml"],'
    echo '         "env": {'
    echo '           "FRAPPE_BASE_URL": "https://your-frappe-instance.com",'
    echo '           "FRAPPE_API_KEY": "your_api_key",'
    echo '           "FRAPPE_API_SECRET": "your_api_secret"'
    echo '         }'
    echo '       }'
    echo '     }'
    echo '   }'
    echo ""
    echo -e "${YELLOW}   For Claude Desktop:${NC}"
    echo "   ~/Library/Application Support/Claude/claude_desktop_config.json"
    echo '   (same format as above)'
    echo ""
    echo "3. Restart your MCP client (Cursor/Claude Desktop)"
    echo ""
    echo -e "${BLUE}Documentation:${NC}"
    echo "   https://github.com/${REPO}"
    echo ""
    echo -e "${BLUE}Verify installation:${NC}"
    echo "   ${INSTALL_DIR}/${BINARY_NAME} --help"
    echo ""
}

# Cleanup
cleanup() {
    if [ -n "$temp_dir" ] && [ -d "$temp_dir" ]; then
        rm -rf "$temp_dir"
    fi
}

# Main installation flow
main() {
    echo ""
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║                                                       ║${NC}"
    echo -e "${BLUE}║        Frappe MCP Server Installation Script         ║${NC}"
    echo -e "${BLUE}║                                                       ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════╝${NC}"
    echo ""
    
    trap cleanup EXIT
    
    detect_platform
    get_latest_version
    download_binary
    install_binary
    setup_config
    show_next_steps
}

# Run installation
main "$@"

