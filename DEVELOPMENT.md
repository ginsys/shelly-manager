# Shelly Manager - Development Guide

## Quick Start

### Prerequisites
- Go 1.23+
- Node.js 18+
- SQLite (for development)

### 1. Clone and Setup
```bash
git clone https://github.com/ginsys/shelly-manager.git
cd shelly-manager
```

### 2. Start Development Servers

#### Option A: Use Development Script (Recommended)
```bash
./scripts/dev-start.sh
```

This will:
- Build the backend
- Start backend on port 8080
- Start frontend on port 5173
- Display logs in `logs/` directory

To stop servers:
```bash
./scripts/dev-stop.sh
```

#### Option B: Manual Startup

**Terminal 1 - Backend:**
```bash
# Build backend
CGO_ENABLED=1 go build -o bin/shelly-manager ./cmd/shelly-manager

# Start backend
./bin/shelly-manager --config configs/development.yaml server
```

**Terminal 2 - Frontend:**
```bash
cd ui
npm install
npm run dev
```

### 3. Access the Application
- **Frontend**: http://localhost:5173
- **Backend API**: http://localhost:8080/api/v1
- **Health Check**: http://localhost:8080/healthz
- **Metrics**: http://localhost:8080/metrics/dashboard

---

## Development Workflow

### Working on an Issue

1. Pick an issue from [GitHub Issues](https://github.com/ginsys/shelly-manager/issues)
2. Create a feature branch from `main`:
   ```bash
   git checkout main
   git pull origin main
   git checkout -b fix/74-partial-updates
   ```
3. Make changes and commit:
   ```bash
   make test-ci
   git add <files>
   git commit -m "fix(api): support partial device updates

   Closes #74"
   ```
4. Push and create a PR:
   ```bash
   git push -u origin fix/74-partial-updates
   gh pr create --base main
   ```
5. After the PR is merged, clean up:
   ```bash
   git checkout main
   git pull origin main
   git branch -d fix/74-partial-updates
   ```

See `CONTRIBUTING.md` for branch naming conventions and full PR guidelines.

### Handling Dependabot PRs

Use `@dependabot` commands in PR comments (not `gh pr close`):

| Command | Effect |
|---------|--------|
| `@dependabot close` | Close PR, prevent recreation |
| `@dependabot ignore this dependency` | Ignore permanently |
| `@dependabot ignore this major version` | Ignore major updates |
| `@dependabot rebase` | Rebase the PR |
| `@dependabot recreate` | Recreate from scratch |

When deferring: comment `@dependabot close` with an explanation.

---

## Configuration

### Backend Configuration
Edit `configs/development.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

security:
  admin_api_key: "dev-admin-key-12345"  # Change this!
  
database:
  provider: sqlite
  path: data/shelly-dev.db
```

### Frontend Configuration
Edit `ui/.env.local`:

```env
VITE_API_BASE=http://localhost:8080/api/v1
VITE_ADMIN_KEY=dev-admin-key-12345  # Must match backend!
```

**Important**: The `VITE_ADMIN_KEY` must match the backend's `security.admin_api_key`

---

## API Authentication

### Protected Endpoints
The following endpoints require the admin API key:
- `/api/v1/export/*` - All export endpoints
- `/api/v1/import/*` - All import endpoints
- `/api/v1/admin/*` - Admin operations

### Testing with curl
```bash
# Without auth (public endpoints)
curl http://localhost:8080/api/v1/devices

# With auth (protected endpoints)
curl -H "Authorization: Bearer dev-admin-key-12345" \
  http://localhost:8080/api/v1/export/plugins
```

---

## Common Development Tasks

### Running Tests
```bash
# Backend tests
make test

# Frontend unit tests
cd ui && npm test

# E2E tests
cd ui && npm run test:e2e
```

### Building for Production
```bash
# Build backend
make build

# Build frontend
cd ui && npm run build
```

### Linting
```bash
# Go linting
make lint

# Frontend linting (if configured)
cd ui && npm run lint
```

### Database Management
```bash
# Reset development database
rm data/shelly-dev.db
./bin/shelly-manager --config configs/development.yaml server
# Database will be auto-created with migrations
```

---

## Troubleshooting

### "Suspicious user agent detected" Error
**Fixed in latest version!** The User-Agent validation now allows standard HTTP clients (curl, axios, browsers).

If you still see this, ensure you're using the latest code:
- Backend: `internal/api/middleware/validation.go` has been updated
- Frontend: `ui/src/api/client.ts` sets User-Agent header

### "UNAUTHORIZED" Error on Export/Import
This means the admin API key is not configured correctly.

**Check:**
1. Backend config has `security.admin_api_key` set
2. Frontend `.env.local` has matching `VITE_ADMIN_KEY`
3. Both servers were restarted after config changes

### Frontend Can't Connect to Backend
**Check:**
1. Backend is running: `curl http://localhost:8080/healthz`
2. CORS is configured: Check `configs/development.yaml` includes your frontend URL in `security.cors.allowed_origins`
3. Frontend env is correct: Check `ui/.env.local` has correct `VITE_API_BASE`

### Port Already in Use
```bash
# Find what's using the port
lsof -i :8080  # Backend
lsof -i :5173  # Frontend

# Stop the processes
./scripts/dev-stop.sh
```

---

## Project Structure

```
shelly-manager/
├── cmd/                    # Entry points
│   ├── shelly-manager/     # Main API server
│   └── shelly-provisioner/ # Provisioning agent
├── internal/               # Backend code
│   ├── api/               # HTTP handlers & routing
│   ├── database/          # Database layer
│   ├── plugins/           # Plugin system
│   ├── metrics/           # Metrics & monitoring
│   └── ...
├── ui/                    # Frontend (Vue 3 + TypeScript)
│   ├── src/
│   │   ├── api/          # API client
│   │   ├── components/   # Vue components
│   │   ├── pages/        # Page components
│   │   ├── stores/       # Pinia state management
│   │   └── ...
│   └── tests/            # E2E & unit tests
├── configs/              # Configuration files
├── scripts/              # Development scripts
└── deploy/               # Deployment configs
```

---

## Environment Variables

### Backend (via config file or environment)
```bash
SHELLY_DATABASE_PROVIDER=sqlite
SHELLY_DATABASE_PATH=data/shelly.db
SHELLY_HTTP_PORT=8080
SHELLY_SECURITY_ADMIN_API_KEY=your-key-here
```

### Frontend (via .env.local)
```bash
VITE_API_BASE=http://localhost:8080/api/v1
VITE_ADMIN_KEY=your-key-here
VITE_DEV_MODE=true
```

---

## Next Steps

1. **Add Test Data**: Use the API or UI to add some devices
2. **Explore Features**: Try export/import, metrics dashboard, plugin management
3. **Read API Docs**: Check `docs/` directory for API documentation
4. **Contribute**: See `CONTRIBUTING.md` for contribution guidelines

---

## Getting Help

- **Documentation**: See `docs/` directory
- **Issues**: https://github.com/ginsys/shelly-manager/issues
- **API Reference**: http://localhost:8080/api/v1 (with server running)

