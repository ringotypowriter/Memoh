#!/bin/bash

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}   Memoh Docker Compose Deployment${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# Check Docker
if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: Docker is not installed${NC}"
    echo "Please install Docker first:"
    echo "  - Linux: curl -fsSL https://get.docker.com | sh"
    echo "  - macOS: brew install --cask docker"
    echo "  - Windows: https://docs.docker.com/desktop/install/windows-install/"
    echo "  - Official guide: https://docs.docker.com/get-docker/"
    exit 1
fi

# Check Docker Compose
if ! docker compose version &> /dev/null; then
    echo -e "${RED}Error: Docker Compose is not installed or version is too old${NC}"
    echo "Docker Compose v2.0+ is required (bundled with Docker Desktop)"
    echo "  - Linux: sudo apt-get install docker-compose-plugin"
    echo "  - Or follow: https://docs.docker.com/compose/install/"
    exit 1
fi

echo -e "${GREEN}✓ Docker and Docker Compose are installed${NC}"
echo ""


# Check config.toml
if [ ! -f config.toml ]; then
    echo -e "${YELLOW}⚠ config.toml does not exist, creating...${NC}"
    cp docker/config/config.docker.toml config.toml
    echo -e "${GREEN}✓ config.toml created${NC}"
    echo ""
fi

# Prepare data root path for host/containerd compatibility
MEMOH_DATA_ROOT="$(pwd)/.data/memoh"
mkdir -p "${MEMOH_DATA_ROOT}"
export MEMOH_DATA_ROOT
if grep -q '^data_root[[:space:]]*=' config.toml; then
    awk -v path="${MEMOH_DATA_ROOT}" '
        $0 ~ /^data_root[[:space:]]*=/ { print "data_root = \"" path "\""; next }
        { print }
    ' config.toml > config.toml.tmp && mv config.toml.tmp config.toml
fi
echo -e "${GREEN}✓ Data root: ${MEMOH_DATA_ROOT}${NC}"
echo ""

# Prepare container runtime environment
echo -e "${GREEN}Preparing container runtime environment...${NC}"
if sh scripts/containerd-install.sh > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Container runtime environment is ready${NC}"
else
    echo -e "${YELLOW}⚠ Failed to prepare container runtime environment, MCP build may be skipped${NC}"
fi
echo ""

# Build MCP image on host with nerdctl
MCP_IMAGE="docker.io/library/memoh-mcp:latest"
echo -e "${GREEN}Building MCP image on host with nerdctl...${NC}"
if command -v nerdctl &> /dev/null && command -v buildctl &> /dev/null && command -v buildkitd &> /dev/null; then
    if nerdctl build -f docker/Dockerfile.mcp -t "${MCP_IMAGE}" . > /dev/null 2>&1; then
        echo -e "${GREEN}✓ MCP image built successfully (on host)${NC}"
    else
        echo -e "${YELLOW}⚠ MCP image build failed on host, will try to pull at runtime${NC}"
    fi
else
    echo -e "${YELLOW}⚠ nerdctl/buildkit environment not found on host, skipping MCP build${NC}"
fi
echo ""

# Start services
echo -e "${GREEN}Starting services...${NC}"
docker compose up -d

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}   Deployment Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo "Service URLs:"
echo "  - Web UI: http://localhost"
echo "  - API Service: http://localhost:8080"
echo "  - Agent Gateway: http://localhost:8081"
echo ""
echo "View service status:"
echo "  docker compose ps"
echo ""
echo "View logs:"
echo "  docker compose logs -f"
echo ""
echo "Stop services:"
echo "  docker compose down"
echo ""
echo -e "${YELLOW}⚠ First startup may take 1-2 minutes, please be patient${NC}"
echo ""
echo "View detailed documentation: DEPLOYMENT.md"
