import { execFileSync } from 'child_process';

// Defaults match the dedicated E2E stack (docker-compose.e2e.yml in
// vibe-server) so an unset-env local run behaves exactly as before.
// `make e2e` in vibe-server overrides BACKEND_URL to its isolated API
// instance (:8081) so this healthcheck never targets the dev API.
const DB_HOST = process.env.E2E_DB_HOST ?? 'localhost';
const DB_PORT = process.env.E2E_DB_PORT ?? '5433';
const DB_USER = process.env.E2E_DB_USER ?? 'vibe';
const DB_NAME = process.env.E2E_DB_NAME ?? 'vibe_e2e';
const DB_PASSWORD = process.env.E2E_DB_PASSWORD ?? 'vibe_e2e';
const BACKEND_URL = process.env.E2E_BACKEND_HEALTHCHECK_URL ?? 'http://localhost:8080/api/v1/healthcheck';
const MAX_RETRIES = 30;
const RETRY_INTERVAL_MS = 1000;

async function globalSetup() {
  // Reset E2E database
  try {
    execFileSync(
      'psql',
      [
        '-h',
        DB_HOST,
        '-p',
        DB_PORT,
        '-U',
        DB_USER,
        '-d',
        DB_NAME,
        // Kept in sync with db/migrations in vibe-server. This used to include
        // `feature_flags`, a table that doesn't exist in any migration — a
        // single unknown relation makes the whole TRUNCATE statement fail
        // atomically, so every table below was silently left un-truncated on
        // every run (the catch below hid it behind one generic warning) and
        // specs accumulated stale rows across runs instead of starting clean.
        '-c',
        'TRUNCATE users, complexes, courts, court_prices, complex_schedules, bookings, clients, payments, refresh_tokens, blocked_slots, email_verification_tokens, password_reset_tokens, audit_log, booking_link_tokens, webhook_events, failed_refunds, slot_locks, job_locks CASCADE;',
      ],
      {
        stdio: 'pipe',
        env: { ...process.env, PGPASSWORD: DB_PASSWORD },
      },
    );
  } catch (err) {
    console.warn('Warning: Could not truncate E2E database. Make sure it is running.', err);
  }

  // Wait for backend to be ready
  for (let i = 0; i < MAX_RETRIES; i++) {
    try {
      const res = await fetch(BACKEND_URL);
      if (res.ok) {
        console.log('Backend is ready.');
        return;
      }
    } catch {
      // Backend not ready yet
    }
    await new Promise((r) => setTimeout(r, RETRY_INTERVAL_MS));
  }

  throw new Error(`Backend not reachable at ${BACKEND_URL} after ${String(MAX_RETRIES)}s`);
}

export default globalSetup;
