# Install: Docker

## Build the image

```bash
make docker
# or
docker build -t matrix-runtime:local .
```

The image bundles `matrix-runtime` plus `node`/`npm`/`npx`, `python3` and
`pipx`, and runs as a non-root user with a `/var/lib/matrix-runtime` volume.

## Run

```bash
docker run -p 8080:8080 matrix-runtime:local --mode local-dev
```

## Docker Compose

```bash
make compose-up
# or
docker compose -f deploy/docker-compose/docker-compose.yml up --build
```

Copy `deploy/docker-compose/.env.example` to `.env` to set mode, limits, the
hybrid join token and `HF_TOKEN`.

## Verify

```bash
curl http://localhost:8080/v1/health
```
