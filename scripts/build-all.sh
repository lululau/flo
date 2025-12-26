#!/bin/bash

# Build script for flo - Multi-platform binary builder
# Usage: ./scripts/build-all.sh [version]

set -e

VERSION=${1:-"dev"}
BUILD_DIR="dist"
BINARY_NAME="flo"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Building flo v${VERSION} for multiple platforms...${NC}"

# Create build directory
mkdir -p ${BUILD_DIR}

# Build configurations
declare -a PLATFORMS=(
    "darwin/amd64/macos-intel-x64"
    "darwin/arm64/macos-aarch64"
    "linux/amd64/linux-x64"
    "linux/arm64/linux-arm64"
    "windows/amd64/windows-x64"
)

# Build for each platform
for platform in "${PLATFORMS[@]}"; do
    IFS='/' read -r GOOS GOARCH NAME <<< "$platform"
    
    OUTPUT_NAME="${BINARY_NAME}-${VERSION}-${NAME}"
    if [ "$GOOS" = "windows" ]; then
        OUTPUT_NAME="${OUTPUT_NAME}.exe"
    fi
    
    echo -e "${YELLOW}📦 Building for ${GOOS}/${GOARCH} (${NAME})...${NC}"
    
    env GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=0 go build \
        -ldflags="-s -w -X main.version=${VERSION}" \
        -o "${BUILD_DIR}/${OUTPUT_NAME}" \
        ./cmd/aliyun-pipelines-tui
    
    if [ $? -eq 0 ]; then
        SIZE=$(du -h "${BUILD_DIR}/${OUTPUT_NAME}" | cut -f1)
        echo -e "${GREEN}✅ Built ${OUTPUT_NAME} (${SIZE})${NC}"
    else
        echo -e "${RED}❌ Failed to build for ${GOOS}/${GOARCH}${NC}"
        exit 1
    fi
done

echo ""
echo -e "${BLUE}📋 Build Summary:${NC}"
echo "=================="
ls -lh ${BUILD_DIR}/

echo ""
echo -e "${GREEN}🎉 All builds completed successfully!${NC}"
echo -e "${YELLOW}📁 Binaries are available in: ${BUILD_DIR}/${NC}"

# Generate checksums
echo ""
echo -e "${BLUE}🔐 Generating checksums...${NC}"
cd ${BUILD_DIR}
sha256sum * > checksums.txt
echo -e "${GREEN}✅ Checksums saved to ${BUILD_DIR}/checksums.txt${NC}"

echo ""
echo -e "${BLUE}📝 Checksums:${NC}"
cat checksums.txt 