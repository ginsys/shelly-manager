#!/bin/bash
# Development startup script for Shelly Manager
# This script starts both backend and frontend servers for local development

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== Shelly Manager Development Startup ===${NC}\n"

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    echo -e "${RED}Error: Must run from repository root${NC}"
    exit 1
fi

# Build backend
echo -e "${YELLOW}Building backend...${NC}"
CGO_ENABLED=1 go build -o bin/shelly-manager ./cmd/shelly-manager
echo -e "${GREEN}✓ Backend built${NC}\n"

# Create data directories
mkdir -p data/exports data/imports

# Start backend in background
echo -e "${YELLOW}Starting backend server on port 8080...${NC}"
./bin/shelly-manager --config configs/development.yaml server > logs/backend.log 2>&1 &
BACKEND_PID=$!
echo $BACKEND_PID > /tmp/shelly-backend.pid
echo -e "${GREEN}✓ Backend started (PID: $BACKEND_PID)${NC}"

# Wait for backend to be ready
echo -e "${YELLOW}Waiting for backend to be ready...${NC}"
for i in {1..30}; do
    if curl -s http://localhost:8080/healthz > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Backend is ready${NC}\n"
        break
    fi
    if [ $i -eq 30 ]; then
        echo -e "${RED}Error: Backend failed to start${NC}"
        cat logs/backend.log
        kill $BACKEND_PID 2>/dev/null || true
        exit 1
    fi
    sleep 1
done

# Start frontend
echo -e "${YELLOW}Starting frontend dev server on port 5173...${NC}"
cd ui
npm run dev > ../logs/frontend.log 2>&1 &
FRONTEND_PID=$!
echo $FRONTEND_PID > /tmp/shelly-frontend.pid
cd ..
echo -e "${GREEN}✓ Frontend started (PID: $FRONTEND_PID)${NC}\n"

echo -e "${GREEN}=== Development servers running ===${NC}"
echo -e "Backend:  ${YELLOW}http://localhost:8080${NC}"
echo -e "Frontend: ${YELLOW}http://localhost:5173${NC}"
echo -e "API:      ${YELLOW}http://localhost:8080/api/v1${NC}"
echo -e ""
echo -e "Logs:"
echo -e "  Backend:  ${YELLOW}logs/backend.log${NC}"
echo -e "  Frontend: ${YELLOW}logs/frontend.log${NC}"
echo -e ""
echo -e "To stop servers, run: ${YELLOW}./scripts/dev-stop.sh${NC}"
echo -e ""
