FROM golang:1.27-alpine AS builder

# ca-certificates and tzdata are copied into the distroless final stage below;
# golang:*-alpine ships neither by default.
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Matches the CI env (see .github/workflows/backend.yml): the build never
# touches go.mod/go.sum on its own, and a network hiccup mid-resolve should
# fail loudly instead of silently rewriting the lockfile.
ENV GOFLAGS=-mod=readonly

# Dependency layer cached independently of source changes: this RUN only
# reruns when go.mod/go.sum change, not on every source edit.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# RAILWAY_GIT_COMMIT_SHA is a Railway-provided reference variable on any
# deploy triggered by a GitHub commit (docs: variables/reference.md, "Git
# Deployment Variables"). Dockerfile builds only receive Railway variables
# as build args when the Dockerfile opts in with a matching ARG (docs:
# builds/dockerfiles.md, "Using variables at build time") — declaring it here
# is that opt-in. COMMIT then defaults to it, so a GitHub-integration deploy
# stamps the real commit with no --build-arg wired by hand; `docker build`
# with neither arg falls back to "dev". Confirmed against Railway's public
# docs, not against the live Railway project — verify a real deploy actually
# receives a non-"dev" COMMIT before relying on it.
ARG RAILWAY_GIT_COMMIT_SHA=dev
ARG COMMIT=${RAILWAY_GIT_COMMIT_SHA}

# -X main.version=${COMMIT} is a no-op today: cmd/api/version.go declares
# `const appVersion = "1.0.0"`, and `-X` only overrides package-level string
# `var`s, never `const`s, and only ones actually named `main.version`. Left in
# place because BLD-01 mandates this flag and the companion Go PR (OBS-02) is
# expected to turn appVersion into `var version = "dev"` and read it in
# main.go's logger/expvar/Sentry-release wiring — once that lands this line
# starts working with no Dockerfile change. Confirmed locally: the linker
# neither fails nor warns when the symbol doesn't exist, it just does nothing.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w -X main.version=${COMMIT}" \
      -o /app/bin/api ./cmd/api

# distroless/static: no shell, no package manager, no libc — only the
# certs/tzdata/binary copied in below. Runs as `nonroot` (uid 65532) already,
# USER below is kept explicit for auditability.
FROM gcr.io/distroless/static:nonroot

# The application fixes process time at UTC by design (see
# docs/adr/0005-single-timezone.md and DOM-01): all storage and internal
# comparisons happen in UTC, and conversion to a complex's local timezone
# (America/Argentina/Buenos_Aires today) happens only at the presentation
# edge, given the target zone explicitly. /usr/share/zoneinfo is still copied
# below because that edge conversion needs the full IANA database — TZ=UTC
# alone would only satisfy lookups of "UTC" itself.
ENV TZ=UTC

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /app/bin/api /app/api

USER nonroot

EXPOSE 8080

# No HEALTHCHECK: distroless/static has no wget/curl/shell to run one with.
# railway.toml's healthcheckPath already covers readiness for the one
# platform this image deploys to; a Docker-level HEALTHCHECK would be dead
# weight everywhere else this image runs (e.g. `docker run` locally).
CMD ["/app/api"]
