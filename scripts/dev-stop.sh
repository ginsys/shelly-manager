#!/bin/bash
# Stop development servers

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}Stopping Shelly Manager development servers...${NC}\n"

# Stop backend
if [ -f /tmp/shelly-backend.pid ]; then
    BACKEND_PID=$(cat /tmp/shelly-backend.pid)
    if kill $BACKEND_PID 2>/dev/null; then
        echo -e "${GREEN}✓ Backend stopped (PID: $BACKEND_PID)${NC}"
    fi
    rm /tmp/shelly-backend.pid
fi

# Stop frontend
if [ -f /tmp/shelly-frontend.pid ]; then
    FRONTEND_PID=$(cat /tmp/shelly-frontend.pid)
    if kill $FRONTEND_PID 2>/dev/null; then
        echo -e "${GREEN}✓ Frontend stopped (PID: $FRONTEND_PID)${NC}"
    fi
    rm /tmp/shelly-frontend.pid
fi

# Also try pkill as backup
pkill -f "shelly-manager.*server" 2>/dev/null || true
pkill -f "vite" 2>/dev/null || true

echo -e "\n${GREEN}Development servers stopped${NC}"
