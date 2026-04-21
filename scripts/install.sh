#!/usr/bin/env bash
#
# binds installation script
# Usage: curl -fsSL https://raw.githubusercontent.com/IkuTri/binds/main/scripts/install.sh | bash
#

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

REPO="IkuTri/binds"
BINARY="binds"

log_info()    { echo -e "${BLUE}==>${NC} $1"; }
log_success() { echo -e "${GREEN}==>${NC} $1"; }
log_warning() { echo -e "${YELLOW}==>${NC} $1"; }
log_error()   { echo -e "${RED}Error:${NC} $1" >&2; }

detect_platform() {
    local os arch
    case "$(uname -s)" in
        Darwin)  os="darwin" ;;
        Linux)   os="linux" ;;
        *)       log_error "Unsupported OS: $(uname -s)"; exit 1 ;;
    esac
    case "$(uname -m)" in
        x86_64|amd64)    arch="amd64" ;;
        aarch64|arm64)   arch="arm64" ;;
        *)               log_error "Unsupported arch: $(uname -m)"; exit 1 ;;
    esac
    echo "${os}_${arch}"
}

resign_for_macos() {
    [[ "$(uname -s)" != "Darwin" ]] && return 0
    command -v codesign &>/dev/null || return 0
    log_info "Re-signing binary for macOS..."
    codesign --remove-signature "$1" 2>/dev/null || true
    codesign --force --sign - "$1" && log_success "Binary re-signed" || log_warning "Re-sign failed (non-fatal)"
}

install_from_release() {
    local platform=$1
    local tmp_dir=$(mktemp -d)

    log_info "Fetching latest release..."
    local release_json
    if command -v curl &>/dev/null; then
        release_json=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest")
    elif command -v wget &>/dev/null; then
        release_json=$(wget -qO- "https://api.github.com/repos/$REPO/releases/latest")
    else
        log_error "Neither curl nor wget found."
        return 1
    fi

    local version=$(echo "$release_json" | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        log_error "Failed to fetch latest version"
        return 1
    fi
    log_info "Latest version: $version"

    local archive_name="binds_${version#v}_${platform}.tar.gz"
    local download_url="https://github.com/$REPO/releases/download/${version}/${archive_name}"

    if ! echo "$release_json" | grep -Fq "\"name\": \"$archive_name\""; then
        log_warning "No prebuilt binary for $platform. Falling back to source build."
        rm -rf "$tmp_dir"
        return 1
    fi

    log_info "Downloading $archive_name..."
    cd "$tmp_dir"
    if command -v curl &>/dev/null; then
        curl -fsSL -o "$archive_name" "$download_url" || { log_error "Download failed"; rm -rf "$tmp_dir"; return 1; }
    else
        wget -q -O "$archive_name" "$download_url" || { log_error "Download failed"; rm -rf "$tmp_dir"; return 1; }
    fi

    log_info "Extracting..."
    tar -xzf "$archive_name"

    local install_dir
    if [[ -w /usr/local/bin ]]; then
        install_dir="/usr/local/bin"
    else
        install_dir="$HOME/.local/bin"
        mkdir -p "$install_dir"
    fi

    log_info "Installing to $install_dir..."
    local extracted_dir=$(find . -name "binds" -type f | head -1)
    if [[ -z "$extracted_dir" ]]; then
        log_error "Binary not found in archive"
        rm -rf "$tmp_dir"
        return 1
    fi

    chmod +x "$extracted_dir"
    if [[ -w "$install_dir" ]]; then
        mv "$extracted_dir" "$install_dir/$BINARY"
    else
        sudo mv "$extracted_dir" "$install_dir/$BINARY"
    fi

    resign_for_macos "$install_dir/$BINARY"

    # Create bd symlink for compatibility
    rm -f "$install_dir/bd" 2>/dev/null
    if [[ -w "$install_dir" ]]; then
        ln -s "$BINARY" "$install_dir/bd"
    else
        sudo ln -s "$BINARY" "$install_dir/bd"
    fi

    log_success "$BINARY installed to $install_dir/$BINARY"

    if [[ ":$PATH:" != *":$install_dir:"* ]]; then
        log_warning "$install_dir is not in your PATH"
        echo ""
        echo "Add this to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo "  export PATH=\"\$PATH:$install_dir\""
        echo ""
    fi

    cd - >/dev/null || cd "$HOME"
    rm -rf "$tmp_dir"
    return 0
}

build_from_source() {
    if ! command -v go &>/dev/null; then
        log_error "Go is required for source builds. Install from https://go.dev/dl/"
        return 1
    fi

    log_info "Building from source..."
    local tmp_dir=$(mktemp -d)
    cd "$tmp_dir"

    if git clone --depth 1 "https://github.com/$REPO.git"; then
        cd binds
        if go build -o binds ./cmd/binds/; then
            local install_dir
            if [[ -w /usr/local/bin ]]; then
                install_dir="/usr/local/bin"
            else
                install_dir="$HOME/.local/bin"
                mkdir -p "$install_dir"
            fi

            if [[ -w "$install_dir" ]]; then
                mv binds "$install_dir/"
            else
                sudo mv binds "$install_dir/"
            fi

            resign_for_macos "$install_dir/$BINARY"

            rm -f "$install_dir/bd" 2>/dev/null
            if [[ -w "$install_dir" ]]; then
                ln -s "$BINARY" "$install_dir/bd"
            else
                sudo ln -s "$BINARY" "$install_dir/bd"
            fi

            log_success "$BINARY installed to $install_dir/$BINARY"

            if [[ ":$PATH:" != *":$install_dir:"* ]]; then
                log_warning "$install_dir is not in your PATH"
                echo "  export PATH=\"\$PATH:$install_dir\""
            fi

            cd - >/dev/null || cd "$HOME"
            rm -rf "$tmp_dir"
            return 0
        else
            log_error "Build failed"
            cd - >/dev/null || cd "$HOME"
            rm -rf "$tmp_dir"
            return 1
        fi
    else
        log_error "Failed to clone repository"
        rm -rf "$tmp_dir"
        return 1
    fi
}

main() {
    echo ""
    echo "  binds — Agent Coordination & Work Tracking"
    echo "  https://github.com/$REPO"
    echo ""

    local platform=$(detect_platform)
    log_info "Detected platform: $platform"

    # Try prebuilt binary first, fall back to source
    if install_from_release "$platform"; then
        :
    elif build_from_source; then
        :
    else
        log_error "Installation failed. Please report at https://github.com/$REPO/issues"
        exit 1
    fi

    echo ""
    if command -v binds &>/dev/null; then
        log_success "binds is installed and ready!"
        echo ""
        binds version 2>/dev/null || echo "binds (development build)"
        echo ""
        echo "Quick start:"
        echo "  cd your-project"
        echo "  binds init"
        echo "  binds create \"First task\" -p 1"
        echo "  binds ready"
        echo ""
    else
        log_warning "binds was installed but isn't in your PATH yet."
        echo "Restart your shell or run: export PATH=\"\$PATH:\$HOME/.local/bin\""
    fi
}

main "$@"
