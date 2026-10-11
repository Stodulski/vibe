// Railway Infrastructure as Code for the Backend service, environment production.
//
// Replaces backend/railway.toml (Config as Code, which stops working on
// 2026-12-01). This is a partial: it owns only the Backend service. Postgres and
// Redis are not declared here and apply never touches them.
//
// Every variable is declared with preserve(): Railway keeps the value it has, and
// this file holds no secret. A variable left out of `env` is deleted by apply, so
// add new variables here before setting them in Railway.
//
// Preview with `railway config plan` from backend/ (the directory the CLI is
// linked from) and apply only after reading the plan. The SDK is the root
// devDependency `railway`.

import { defineRailway, github, preserve, project, service } from "railway/iac";

export const partial = "Backend";

const REGION = "us-east4-eqdc4a";
const LIMIT = { containers: { cpu: 1, memoryBytes: 1000000000 } };

// --- Backend (the API) ---------------------------------------------------

const backend = service("Backend", {
  source: github("Stodulski/vibe", {
    branch: "main",
    // Service root directory. dockerfilePath is relative to it.
    rootDirectory: "/backend",
    checkSuites: true,
  }),

  build: {
    builder: "DOCKERFILE",
    // Relative to the service root directory (/backend).
    dockerfilePath: "Dockerfile",
    // Gitignore-style, resolved from the REPOSITORY root, not the service root.
    // Only backend/ changes redeploy the API; frontend and landing commits do not
    // rebuild the image or run the migration against production.
    watchPatterns: ["/backend/**"],
  },

  deploy: {
    startCommand: "/app/api",
    // Runs once per deploy, after the image is built and before any container
    // serves. Migrations are compiled into the binary. A non-zero exit stops the
    // deploy and the previous release keeps serving the previous schema. Not retried.
    preDeployCommand: ["/app/api -migrate-only"],
    // Readiness, not liveness: pings Postgres and Redis (2s timeout) and answers
    // 503 only when Postgres is down. /api/v1/livez is the unconditional probe.
    healthcheckPath: "/api/v1/healthcheck",
    // Seconds Railway keeps probing before it marks the replica unhealthy. The
    // boot opens Postgres, pings Redis, then listens.
    healthcheckTimeout: 120,
    // restartPolicyType is ON_FAILURE, Railway's default, which it reads back as
    // unset; declaring it leaves the plan with a permanent diff.
    restartPolicyMaxRetries: 3,
    // Production settings that were never in railway.toml. Declared so the plan
    // shows no drift for them.
    runtime: "V2",
    multiRegionConfig: { [REGION]: { numReplicas: 1 } },
    limitOverride: LIMIT,
    ipv6EgressEnabled: false,
    useLegacyStacker: false,
  },

  networking: {
    privateNetworkEndpoint: "padel-server",
  },

  // railway.toml [service] internalPort = 8080. The IaC model has no service-level
  // port; the custom domain target port is the only port setting, and it is 8080
  // in production. Verify in the plan.
  domains: [{ domain: "api.vibe.com.ar", port: 8080 }],

  // Values stay in Railway. preserve() keeps each one as it is and keeps this
  // file from holding any secret.
  env: {
    // Public URLs, cookies, runtime
    BACKEND_URL: preserve(),
    COOKIE_DOMAIN: preserve(),
    ENV: preserve(),
    FRONTEND_URL: preserve(),
    PORT: preserve(),
    PUBLIC_URL: preserve(),
    PPROF_ENABLED: preserve(),
    REQUEST_LOG_SAMPLE: preserve(),
    SENTRY_DSN: preserve(),
    SENTRY_RELEASE: preserve(),
    TRUSTED_PROXIES: preserve(),
    // Database and cache. Values may be references to the Postgres/Redis nodes.
    DATABASE_URL: preserve(),
    DB_AUTO_MIGRATE: preserve(),
    DB_MIGRATOR_URL: preserve(),
    DB_STATEMENT_TIMEOUT: preserve(),
    REDIS_URL: preserve(),
    // Bookings
    BOOKING_GRACE_PERIOD: preserve(),
    BOOKING_LINK_TOKEN_BUFFER: preserve(),
    BOOKING_PAYMENT_EXPIRY: preserve(),
    BOOKING_SLOT_LOCK_TTL: preserve(),
    LEADS_ABANDONED_WEBHOOK_TOKEN: preserve(),
    LEADS_ABANDONED_WEBHOOK_URL: preserve(),
    // Auth and secrets
    JWT_SECRET: preserve(),
    GOOGLE_OAUTH_CLIENT_ID: preserve(),
    GOOGLE_OAUTH_CLIENT_SECRET: preserve(),
    GOOGLE_MAPS_API: preserve(),
    TURNSTILE_SECRET_KEY: preserve(),
    VIBE_APP_PASSWORD: preserve(),
    // Payments (Mercado Pago)
    MP_ACCESS_TOKEN: preserve(),
    MP_APP_ID: preserve(),
    MP_CLIENT_SECRET: preserve(),
    MP_CREDENTIAL_KEYS: preserve(),
    MP_WEBHOOK_SECRET: preserve(),
    // Messaging (Brevo, WhatsApp)
    BREVO_API_KEY: preserve(),
    BREVO_SENDER: preserve(),
    WHATSAPP_APP_SECRET: preserve(),
    WHATSAPP_PHONE_NUMBER_ID: preserve(),
    WHATSAPP_TOKENs: preserve(),
    WHATSAPP_VERIFY_TOKEN: preserve(),
    // Object storage (Cloudflare R2)
    R2_ACCESS_KEY: preserve(),
    R2_ACCOUNT_ID: preserve(),
    R2_BUCKET_NAME: preserve(),
    R2_PUBLIC_URL: preserve(),
    R2_SECRET_KEY: preserve(),
  },
});

export default defineRailway(() =>
  project("Vibe", {
    resources: [backend],
  }),
);
