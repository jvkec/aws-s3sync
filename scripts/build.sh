#!/bin/bash

# s3sync cross-platform build script
# builds binaries for multiple operating systems and architectures

set -e

# project info
project_name="s3sync"
version=${1:-"v1.0.0"}
build_dir="builds"
main_package="./cmd/s3sync"

# colors for output
red='\033[0;31m'
green='\033[0;32m'
yellow='\033[1;33m'
blue='\033[0;34m'
nc='\033[0m' # no color

echo -e "${blue}🔧 building ${project_name} ${version}${nc}"

# create build directory
rm -rf ${build_dir}
mkdir -p ${build_dir}

# build info
build_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
git_commit=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# ldflags for build info and optimization
ldflags="-s -w -X main.version=${version} -X main.buildTime=${build_time} -X main.gitCommit=${git_commit}"

# platforms to build for
platforms=(
    "windows/amd64"
    "windows/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
    "linux/386"
)

echo -e "${yellow}📦 building for ${#platforms[@]} platforms...${nc}"

for platform in "${platforms[@]}"; do
    os=${platform%/*}
    arch=${platform#*/}
    
    output_name="${project_name}-${os}-${arch}"
    if [ $os = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    output_path="${build_dir}/${output_name}"
    
    echo -e "${blue}building ${platform}...${nc}"
    
    env GOOS=$os GOARCH=$arch go build \
        -ldflags="${ldflags}" \
        -o ${output_path} \
        ${main_package}
    
    if [ $? -eq 0 ]; then
        file_size=$(ls -lh ${output_path} | awk '{print $5}')
        echo -e "${green}✅ ${platform} (${file_size})${nc}"
    else
        echo -e "${red}❌ failed to build ${platform}${nc}"
        exit 1
    fi
done

echo ""
echo -e "${green}🎉 build completed successfully!${nc}"
echo -e "${blue}📁 binaries created in: ${build_dir}/${nc}"

# list all created binaries
echo ""
echo "created binaries:"
ls -lh ${build_dir}/ | grep -v "^total" | while read line; do
    echo "  $line"
done

# create checksums
echo ""
echo -e "${yellow}🔐 generating checksums...${nc}"
cd ${build_dir}
sha256sum * > checksums.txt
echo -e "${green}✅ checksums saved to checksums.txt${nc}"
cd ..

echo ""
echo -e "${blue}🚀 ready for distribution!${nc}" 