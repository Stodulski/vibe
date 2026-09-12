# .env is gitignored and absent on a fresh clone and in CI, so this include is
# optional: without the leading dash every target below fails before it runs.
-include .env
export

# ==================================================================================== #
# HELPERS
# ==================================================================================== #

.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## run: run the API server
.PHONY: run
run:
	@go run ./cmd/api

## tidy: tidy dependencies and modernize code
.PHONY: tidy
tidy:
	go mod tidy -v
	go fix ./...

## build: build the API server binary
.PHONY: build
build:
	CGO_ENABLED=0 go build -o ./bin/api ./cmd/api

## test: run all tests
.PHONY: test
test:
	go test -race -shuffle=on -v -count=1 ./...

## test/cover: Run tests with coverage report and open in browser
.PHONY: test/cover
test/cover:
	@echo "Running tests with coverage..."
	go test -race -shuffle=on -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out
	@echo "\nCoverage report generated: coverage.out"
	@echo "Run 'go tool cover -html=coverage.out' to view in browser"

## test/cover/html: Run tests and open HTML coverage report
.PHONY: test/cover/html
test/cover/html: test/cover
	go tool cover -html=coverage.out -o coverage.html
	@echo "HTML report: coverage.html"

## lint: run linter
.PHONY: lint
lint:
	golangci-lint run

## lint/openapi: check the embedded OpenAPI document loads and validates.
# Route-vs-document parity (every registered route documented, and vice
# versa) needs the application's route table and lives in
# cmd/api/openapi_sync_test.go, run by `make test`.
.PHONY: lint/openapi
lint/openapi:
	go run ./cmd/openapicheck

## audit: verify dependencies and check for vulnerabilities
# govulncheck comes from the `tool` directive in go.mod, not from $PATH. A bare
# `govulncheck` needs a manual `go install` that nobody does on a clean checkout
# or on a fresh CI runner, and make answers a missing binary with exit 127 — a
# security gate that has never once looked at the code. `go tool` builds the
# pinned version from the module graph, so the check runs everywhere and reports
# the same thing everywhere.
#
# Exit code: govulncheck exits 0 when the only findings sit in required modules
# that no reachable symbol calls, and non-zero when your code actually calls a
# vulnerable symbol. So this gate is red exactly when it should be, and does not
# need a `|| true` that would turn it back off.
.PHONY: audit
audit:
	go mod verify
	go tool govulncheck ./...

# ==================================================================================== #
# DATABASE
# ==================================================================================== #

## migrate-up: run all up migrations
.PHONY: migrate-up
migrate-up:
	goose -dir ./db/migrations postgres "$(DATABASE_URL)" up

## migrate-down: run all down migrations (confirm required)
.PHONY: migrate-down
migrate-down: confirm
	goose -dir ./db/migrations postgres "$(DATABASE_URL)" down

## migrate-create: create a new empty migration file ("make migrate-create name=add_foo")
.PHONY: migrate-create
migrate-create:
	goose -dir ./db/migrations create $(name) sql

## migrate-embedded: apply migrations with the API binary instead of the goose CLI (what Railway's pre-deploy command runs)
# Same chain, same goose_db_version table, no goose on PATH: the .sql files are
# compiled into the binary. This target exists so the deploy path can be run
# locally rather than first observed in production — `make migrate-up` is still
# the everyday one, and neither replaces the other.
#
# `go run` and not ./bin/api, so it migrates with the migrations in the working
# tree rather than with whatever a stale build happened to embed.
.PHONY: migrate-embedded
migrate-embedded:
	DB_MIGRATOR_URL="$(DATABASE_URL)" go run ./cmd/api -migrate-only

## sqlc: generate sqlc code
.PHONY: sqlc
sqlc:
	sqlc generate

## vet/sqlc: vet SQL queries against a live, migrated schema (needs DATABASE_URL; see sqlc.vet.yaml)
# Separate config from `sqlc` above: sqlc.vet.yaml carries the sqlc/db-prepare
# rule and its required `database.uri`, which sqlc.yaml deliberately does not
# (see sqlc.vet.yaml's header comment — adding it there made `sqlc generate`
# depend on whichever Postgres happens to be reachable).
.PHONY: vet/sqlc
vet/sqlc:
	sqlc vet -f sqlc.vet.yaml

## seed: run database seed
.PHONY: seed
seed:
	# There is no cmd/seed; the benchmark seed is plain SQL. Never point this
	# at production: it inserts owners, complexes and ~18k bookings.
	psql "$(DATABASE_URL)" -v ON_ERROR_STOP=1 -f db/seed_benchmark.sql

## backup: Run database backup
.PHONY: backup
backup:
	@./scripts/backup.sh

# ==================================================================================== #
# DOCKER
# ==================================================================================== #

## docker-up: start docker containers
.PHONY: docker-up
docker-up:
	docker compose up -d

## docker-down: stop docker containers
.PHONY: docker-down
docker-down:
	docker compose down

# ==================================================================================== #
# E2E TESTING
# ==================================================================================== #

E2E_DSN = postgres://vibe:vibe_e2e@localhost:5433/vibe_e2e?sslmode=disable

# docker-compose.e2e.yml's redis service, host port 6380 (never the dev
# instance's 6379). No password: the compose service sets none.
E2E_REDIS_URL = redis://localhost:6380/0

## e2e-db-up: start E2E test database and run migrations
.PHONY: e2e-db-up
e2e-db-up:
	docker compose -f docker-compose.e2e.yml up -d --wait
	goose -dir ./db/migrations postgres "$(E2E_DSN)" up
	$(MAKE) e2e-db-roles

# The two roles the schema creates, given the ability to log in.
#
# The schema creates them NOLOGIN and with no password, deliberately: roles are
# cluster-level, a password in a migration is a password in git, and the
# ability to open a connection is the only part of a role that needs a secret.
# On a real deployment the equivalent of the two lines below is run once, by
# hand, with real secrets. Here the passwords are the same throwaway strings as
# the E2E database's own, which is already in this file in plain text.
#
# Idempotent: ALTER ROLE on a role that already has these attributes is a no-op,
# and the || true covers a database the schema has not been applied to yet.
E2E_ADMIN_DSN = postgres://vibe:vibe_e2e@localhost:5433/vibe_e2e?sslmode=disable
E2E_APP_DSN   = postgres://vibe_app:vibe_app_e2e@localhost:5433/vibe_e2e?sslmode=disable

## e2e-db-roles: give the vibe_app / vibe_migrator roles a password on the E2E database
.PHONY: e2e-db-roles
e2e-db-roles:
	@psql "$(E2E_DSN)" -X -q \
		-c "ALTER ROLE vibe_app LOGIN PASSWORD 'vibe_app_e2e'" \
		-c "ALTER ROLE vibe_migrator LOGIN PASSWORD 'vibe_migrator_e2e'" \
		|| echo "e2e-db-roles: vibe_app/vibe_migrator do not exist yet (schema not applied); skipping"

## e2e-db-down: stop E2E test database
.PHONY: e2e-db-down
e2e-db-down:
	docker compose -f docker-compose.e2e.yml down -v

## test/integration: run integration tests against real database
# ./... rather than ./cmd/api/: integration tests also live in internal/data,
# and a package that is never named is a test that never runs.
#
# -p 1 because every package shares the one E2E database and cleanupDB
# TRUNCATEs across it. Without it Go runs packages concurrently, cmd/api wipes
# internal/data's fixtures mid-test, and the suite fails on a different test
# each run — which teaches everyone to re-run it rather than read it. A gate
# nobody believes is a gate nobody has.
.PHONY: test/integration
# Only the packages that carry integration files. The tag makes every package
# compile again, so `./...` re-ran the whole unit suite a second time, serially
# (`-p 1` keeps the packages that share the database from stepping on each
# other) and under the race detector, which the unit target already applies.
# TestGoFiles AND XTestGoFiles: an integration file next to a store now says
# `package <x>store_test` rather than `package <x>store`, because it imports
# internal/data/datatest, which imports the store it is testing — an in-package
# test cannot. Those files are XTestGoFiles, and listing only TestGoFiles found
# three packages out of thirteen while reporting success for the rest.
INTEGRATION_PKGS := $(shell go list -tags integration -f '{{.ImportPath}}{{range .TestGoFiles}} {{.}}{{end}}{{range .XTestGoFiles}} {{.}}{{end}}' ./... | grep _integration_test.go | cut -d' ' -f1)

test/integration:
	DATABASE_URL="$(E2E_DSN)" \
	DATABASE_ADMIN_URL="$(E2E_ADMIN_DSN)" \
	DATABASE_APP_URL="$(E2E_APP_DSN)" \
	REDIS_URL="$(E2E_REDIS_URL)" \
	JWT_SECRET="e2e-test-secret-key-32-bytes-long!" \
	go test -tags integration -race -shuffle=on -v -count=1 -p 1 $(INTEGRATION_PKGS)

## test/security: run security tests against real database
.PHONY: test/security
test/security:
	DATABASE_URL="$(E2E_DSN)" \
	JWT_SECRET="e2e-test-secret-key-32-bytes-long!" \
	go test -tags integration -race -v -run TestSecurity -count=1 -p 1 ./...

## e2e: run the client Playwright E2E suite against an isolated stack (db :5433, redis :6380, api :8081, client :5174) — never touches the dev servers or dev DB
.PHONY: e2e
e2e: e2e-db-up
	@docker compose -f docker-compose.e2e.yml up -d --wait redis
	@./scripts/e2e-run.sh

## test/load: run load tests with Artillery (requires the API running on port 8080)
# The scenario file lives in this repo at tests/load/api-load.yml. The old
# recipe hopped through `../frontend` into `../backend`, a layout this project
# has never had, so the target could only ever fail — while `make help` kept
# advertising it.
.PHONY: test/load
test/load:
	npx artillery run ./tests/load/api-load.yml
