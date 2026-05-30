# Local Development Guide

This document describes how to run MatrixHub frontend and backend services locally.

## Prerequisites

- Go 1.23+
- Node.js 18+
- pnpm 8+
- Docker

## Quick Start

### Local Development

If you need to modify code and debug locally, you can start services manually.

#### 1. Start MySQL Database

```bash
docker run -d \
  --name matrixhub-mysql \
  -e MYSQL_ROOT_PASSWORD=password \
  -e MYSQL_DATABASE=matrixhub \
  -e MYSQL_USER=matrixhub \
  -e MYSQL_PASSWORD=changeme \
  -p 3306:3306 \
  mysql:8.4
```

#### 2. Configure Environment Variables

```bash
export MATRIXHUB_DATABASE_DSN="matrixhub:changeme@tcp(127.0.0.1:3306)/matrixhub?charset=utf8mb4&multiStatements=true&parseTime=true"
```

#### 3. Start Backend Service

```bash
# Use default config file
go run ./cmd/matrixhub apiserver

# Or specify config file
go run ./cmd/matrixhub apiserver -c config/dev-config.yaml
```

#### 4. Start Frontend Service

```bash
cd ui
pnpm install  # Install dependencies on first run
pnpm dev
```

The frontend dev server will start at http://localhost:5173 and automatically proxy API requests to the backend.

## Using Makefile

The project provides convenient Makefile commands:

**⚠️ Important**: Before using `local-run` and `local-run-api` commands, **you must start MySQL database first**.

```bash
# 1. Start MySQL database (if not already running)
docker run -d \
  --name matrixhub-mysql \
  -e MYSQL_ROOT_PASSWORD=password \
  -e MYSQL_DATABASE=matrixhub \
  -e MYSQL_USER=matrixhub \
  -e MYSQL_PASSWORD=changeme \
  -p 3306:3306 \
  mysql:8.4

# 2. Run locally (frontend + backend)
make local-run

# Or run separately:
make local-run-api   # Run backend API server only
make local-run-web   # Run frontend only
```

**Tips**:
- `local-run-web` does not depend on MySQL and can run independently
- `local-run-api` and `local-run` require MySQL to be running
- If environment variables are not configured, ensure the database DSN in `config/config.yaml` is correct

## Configuration

### Environment Variables

`config.yaml` supports configuring database connection via environment variables:

```bash
export MATRIXHUB_DATABASE_DSN="matrixhub:changeme@tcp(127.0.0.1:3306)/matrixhub?charset=utf8mb4&multiStatements=true&parseTime=true"
```

### Frontend Proxy Configuration

The frontend is configured with Vite proxy, which automatically forwards `/api/*` and `/apis/*` requests to the backend server (http://127.0.0.1:3001) in development mode, no additional configuration needed.

Configuration file: `ui/vite.config.ts`

## Accessing the Application

- **Frontend UI**: http://localhost:5173
- **Backend API**: http://localhost:3001
- **API Health Check**: http://localhost:3001/health

## Development Tips

### Database

1. **First Run**: Setting `database.migrate: true` will automatically create database tables
2. **Debug Mode**: Setting `debug: true` shows detailed SQL logs
3. **Data Persistence**: Docker Compose uses named volumes, data persists after container restart

### Frontend

1. **Hot Reload**: Code changes automatically refresh the browser
2. **API Proxy**: No need to handle CORS in development mode
3. **TypeScript**: Use `pnpm typecheck` for type checking

### Backend

1. **Code Changes**: Requires manual service restart
2. **Dependency Management**: Use `go mod tidy` to organize dependencies
3. **Configuration Validation**: Config file is automatically validated on startup

## Troubleshooting

### MySQL Connection Failed

```bash
# Check MySQL container status
docker ps | grep matrixhub-mysql

# View MySQL logs
docker logs matrixhub-mysql

# Restart MySQL
docker restart matrixhub-mysql
```

### Port Conflicts

If ports are occupied, you can modify:

**Backend Port**:

Modify `apiServer.port` in `config.yaml`

**Frontend Port**:
```bash
cd ui
pnpm dev --port 3000
```

### Dependency Issues

**Go Dependencies**:
```bash
go mod tidy
go mod download
```

**Frontend Dependencies**:
```bash
cd ui
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

### Generate HTTP Client SDK

When swagger API definitions change, regenerate the HTTP client SDK for testing:

```bash
make gen_openapi_sdk
```

Output: `test/client` directory.
