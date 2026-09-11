#!/usr/bin/env bash
# Drives the client Playwright E2E suite against a fully isolated stack:
# vibe-e2e db (:5433), vibe-e2e redis (:6380), a
# purpose-built API instance (:8081) bound to both, and the client's own
# `--mode e2e` production build served by `vite preview` (:5174, started/stopped by Playwright itself via
# its `webServer` config).
#
# Never touches the developer's running dev API (:8080), dev Vite (:5173),
# dev Postgres (vibe-db-1) or dev Redis (vibe-redis-1) —
# every port and every env var below is E2E-only. Invoked via `make e2e`.
#
# Rate limiting is disabled on this instance (-limiter-enabled=false): the
# auth ceiling is a hardcoded 1 request/6s with a burst of 10
# (internal/middleware/ratelimit.go authCeiling), and Playwright's default
# local run spins up several workers that each log in independently —
# enough concurrent /auth/login calls to blow through that burst and turn
# genuine product behaviour into a wall of unrelated 429s. This instance is
# localhost-only and ephemeral, so disabling it here doesn't test anything
# the production rate limiter needs to prove.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLIENT_DIR="${CLIENT_DIR:-$ROOT_DIR/../frontend}"
PNPM="${PNPM:-pnpm}"

E2E_API_PORT="${E2E_API_PORT:-8081}"
E2E_CLIENT_PORT="${E2E_CLIENT_PORT:-5174}"
E2E_DB_PORT="${E2E_DB_PORT:-5433}"
E2E_REDIS_PORT="${E2E_REDIS_PORT:-6380}"
# One key for both sides. The API decrypts MercadoPago credentials with it,
# and the client fixtures seal their fake credential with it (via
# cmd/mpcredkey). `make` exports the developer's .env, whose real key would
# otherwise reach the Playwright process and seal a token this API cannot
# read — payments_enabled false, every public booking spec failing.
E2E_MP_CREDENTIAL_KEYS="e2e:MDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDAwMDA="

# The client and API ports must be free, and not merely "something answers
# there". Playwright reuses whatever already listens on the client port, so a
# Vite dev server from an unrelated project on 5174 silently became the app
# under test once and every spec failed against a stranger's page. Walk up
# from the default until a port is free, and say which one was taken.
port_in_use() { ss -ltn 2>/dev/null | awk '{print $4}' | grep -Eq "[:.]$1\$"; }
pick_free_port() {
  local port="$1" label="$2"
  local tries=0
  while port_in_use "$port"; do
    echo "[e2e] $label port $port is in use by another process; trying $((port + 1))" >&2
    port=$((port + 1)); tries=$((tries + 1))
    if [ "$tries" -gt 20 ]; then
      echo "[e2e] no free $label port near $1" >&2
      exit 1
    fi
  done
  echo "$port"
}
E2E_API_PORT="$(pick_free_port "$E2E_API_PORT" API)"
E2E_CLIENT_PORT="$(pick_free_port "$E2E_CLIENT_PORT" client)"

E2E_DSN="postgres://vibe:vibe_e2e@localhost:${E2E_DB_PORT}/vibe_e2e?sslmode=disable"
API_BASE_URL="http://localhost:${E2E_API_PORT}"
CLIENT_BASE_URL="http://localhost:${E2E_CLIENT_PORT}"

if [ ! -d "$CLIENT_DIR" ]; then
  echo "[e2e] frontend not found at $CLIENT_DIR — set CLIENT_DIR to override" >&2
  exit 1
fi

echo "[e2e] building API binary..."
CGO_ENABLED=0 go build -o "$ROOT_DIR/bin/vibe-api-e2e" "$ROOT_DIR/cmd/api"
# The public-booking fixtures seal a fake MercadoPago credential with this
# tool; built once here rather than `go run` from inside the test process,
# which compiled it on every call and, on a cold CI runner, took long enough
# for a second worker's test to load the storefront before it was connected.
CGO_ENABLED=0 go build -o "$ROOT_DIR/bin/vibe-mpcredkey-e2e" "$ROOT_DIR/cmd/mpcredkey"

API_PID=""
cleanup() {
  if [ -n "$API_PID" ] && kill -0 "$API_PID" 2>/dev/null; then
    echo "[e2e] stopping API (pid $API_PID)..."
    kill "$API_PID" 2>/dev/null || true
    wait "$API_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

echo "[e2e] starting API on :${E2E_API_PORT} against E2E db (:${E2E_DB_PORT}) and E2E redis (:${E2E_REDIS_PORT})..."
# The Makefile does `-include .env; export`, so if the developer has a real
# .env this script's parent (make) environment may already carry their real
# secrets. Every var that matters for isolation or that could reach a real
# external service is set here explicitly (overriding anything inherited),
# so this instance never touches the dev DB/Redis and never emails, WhatsApps
# or charges anyone for real — construct-a-minimal-env, not copy-the-real-one.
DATABASE_URL="$E2E_DSN" \
JWT_SECRET="e2e-test-secret-key-32-bytes-long!" \
ENV=development \
FRONTEND_URL="$CLIENT_BASE_URL" \
BACKEND_URL="$API_BASE_URL" \
REDIS_URL="redis://localhost:${E2E_REDIS_PORT}" \
COOKIE_DOMAIN="" \
MP_ACCESS_TOKEN="" MP_WEBHOOK_SECRET="" MP_APP_ID="" MP_CLIENT_SECRET="" \
MP_CREDENTIAL_KEYS="$E2E_MP_CREDENTIAL_KEYS" \
WHATSAPP_TOKEN="" WHATSAPP_PHONE_NUMBER_ID="" WHATSAPP_VERIFY_TOKEN="" WHATSAPP_APP_SECRET="" \
BREVO_API_KEY="" SMTP_HOST="" SMTP_USERNAME="" SMTP_PASSWORD="" \
TURNSTILE_SECRET_KEY="1x0000000000000000000000000000000AA" \
SENTRY_DSN="" \
R2_ACCOUNT_ID="" R2_ACCESS_KEY="" R2_SECRET_KEY="" \
GOOGLE_MAPS_API="" \
LEADS_ABANDONED_WEBHOOK_URL="" LEADS_ABANDONED_WEBHOOK_TOKEN="" \
"$ROOT_DIR/bin/vibe-api-e2e" -port "$E2E_API_PORT" -limiter-enabled=false &
API_PID=$!

echo "[e2e] waiting for API healthcheck..."
for _ in $(seq 1 30); do
  if curl -sf "${API_BASE_URL}/api/v1/healthcheck" >/dev/null 2>&1; then
    echo "[e2e] API ready."
    break
  fi
  if ! kill -0 "$API_PID" 2>/dev/null; then
    echo "[e2e] API process exited before becoming healthy." >&2
    exit 1
  fi
  sleep 1
done

if ! curl -sf "${API_BASE_URL}/api/v1/healthcheck" >/dev/null 2>&1; then
  echo "[e2e] API never became healthy at ${API_BASE_URL}." >&2
  exit 1
fi

# Two workers by default: the suite is green in isolation but a few public
# booking specs time out when eight workers fight a four-core laptop for CPU
# with the API, Vite and the browser. Override with PLAYWRIGHT_WORKERS.
#
# Do not put a comment inside the env-assignment chain below: a comment after
# a backslash continuation ends the chain, and Playwright then runs with no
# environment at all — against the developer's own client, API and database.
echo "[e2e] running pnpm test:e2e in $CLIENT_DIR..."
set +e
(
  cd "$CLIENT_DIR" && \
  VITE_API_PROXY_TARGET="$API_BASE_URL" \
  E2E_BASE_URL="$CLIENT_BASE_URL" \
  E2E_BACKEND_HEALTHCHECK_URL="${API_BASE_URL}/api/v1/healthcheck" \
  E2E_DB_HOST="localhost" \
  E2E_DB_PORT="$E2E_DB_PORT" \
  E2E_DB_USER="vibe" \
  E2E_DB_NAME="vibe_e2e" \
  E2E_DB_PASSWORD="vibe_e2e" \
  PLAYWRIGHT_BASE_URL="$CLIENT_BASE_URL" \
  PLAYWRIGHT_WEB_SERVER_COMMAND="pnpm exec vite build --mode e2e --outDir dist-e2e && pnpm exec vite preview --mode e2e --outDir dist-e2e --port ${E2E_CLIENT_PORT} --strictPort" \
  MP_CREDENTIAL_KEYS="$E2E_MP_CREDENTIAL_KEYS" \
  VITE_TURNSTILE_SITE_KEY="1x00000000000000000000AA" \
  SERVER_DIR="$ROOT_DIR" \
  E2E_MPCREDKEY_BIN="$ROOT_DIR/bin/vibe-mpcredkey-e2e" \
  "$PNPM" test:e2e --workers="${PLAYWRIGHT_WORKERS:-2}"
)
STATUS=$?
set -e

exit $STATUS
