#!/bin/bash

# s3sync installation script
# installs the latest version of s3sync for unix-like systems

set -e

# configuration
project_name="s3sync"
github_repo="jvkec/aws-s3-simple-sync"
install_dir="/usr/local/bin"
tmp_dir="/tmp/s3sync-install"

# colors
red='\033[0;31m'
green='\033[0;32m'
yellow='\033[1;33m'
blue='\033[0;34m'
nc='\033[0m'

# helper functions
log() {
    echo -e "${blue}[info]${nc} $1"
}

success() {
    echo -e "${green}[success]${nc} $1"
}

warn() {
    echo -e "${yellow}[warning]${nc} $1"
}

error() {
    echo -e "${red}[error]${nc} $1" >&2
    exit 1
}

# detect os and architecture
detect_platform() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    local arch=$(uname -m)
    
    case $os in
        linux*)
            os="linux"
            ;;
        darwin*)
            os="darwin"
            ;;
        *)
            error "unsupported operating system: $os"
            ;;
    esac
    
    case $arch in
        x86_64|amd64)
            arch="amd64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        i386|i686)
            arch="386"
            ;;
        *)
            error "unsupported architecture: $arch"
            ;;
    esac
    
    echo "${os}/${arch}"
}

# check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# download and install
install_s3sync() {
    local platform=$(detect_platform)
    local os=${platform%/*}
    local arch=${platform#*/}
    
    log "detected platform: ${platform}"
    
    # check for required tools
    if ! command_exists curl && ! command_exists wget; then
        error "either curl or wget is required for installation"
    fi
    
    # create temporary directory
    rm -rf ${tmp_dir}
    mkdir -p ${tmp_dir}
    cd ${tmp_dir}
    
    # determine download tool
    local download_cmd=""
    if command_exists curl; then
        download_cmd="curl -sSL"
    else
        download_cmd="wget -qO-"
    fi
    
    # build binary name
    local binary_name="${project_name}-${os}-${arch}"
    local download_url="https://github.com/${github_repo}/releases/latest/download/${binary_name}"
    
    log "downloading from: ${download_url}"
    
    # download binary
    if ! ${download_cmd} "${download_url}" -o "${binary_name}"; then
        error "failed to download s3sync binary"
    fi
    
    # make executable
    chmod +x "${binary_name}"
    
    # verify binary works
    if ! ./${binary_name} --help >/dev/null 2>&1; then
        error "downloaded binary appears to be corrupted"
    fi
    
    # check if we need sudo for installation
    if [ ! -w "${install_dir}" ]; then
        warn "installation directory ${install_dir} requires sudo access"
        
        if ! command_exists sudo; then
            error "sudo is required but not available"
        fi
        
        sudo mv "${binary_name}" "${install_dir}/${project_name}"
    else
        mv "${binary_name}" "${install_dir}/${project_name}"
    fi
    
    # cleanup
    cd /
    rm -rf ${tmp_dir}
    
    success "s3sync installed to ${install_dir}/${project_name}"
}

# check for existing installation
check_existing() {
    if command_exists s3sync; then
        local current_version=$(s3sync --version 2>/dev/null | head -n1 || echo "unknown")
        warn "s3sync is already installed: ${current_version}"
        
        read -p "do you want to reinstall? [y/n]: " -n 1 -r
        echo
        if [[ ! $reply =~ ^[yy]$ ]]; then
            log "installation cancelled"
            exit 0
        fi
    fi
}

# main installation
main() {
    echo -e "${blue}🚀 s3sync installer${nc}"
    echo "this script will install s3sync to ${install_dir}"
    echo ""
    
    check_existing
    install_s3sync
    
    echo ""
    echo -e "${green}🎉 installation completed!${nc}"
    echo ""
    echo "next steps:"
    echo "  1. run 's3sync setup' to configure aws credentials"
    echo "  2. run 's3sync --help' to see available commands"
    echo "  3. visit https://github.com/${github_repo} for documentation"
    echo ""
    echo "enjoy syncing with s3! 🚀"
}

# run installer
main "$@" 