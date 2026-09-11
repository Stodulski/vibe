import { execFileSync } from 'child_process';

const TABLES = [
  'users',
  'complexes',
  'courts',
  'court_prices',
  'complex_schedules',
  'bookings',
  'clients',
  'payments',
  'refresh_tokens',
  'blocked_slots',
  'email_verification_tokens',
  'password_reset_tokens',
  'audit_log',
  'feature_flags',
  'failed_refunds',
  'slot_locks',
];

export function resetDatabase() {
  const tableList = TABLES.join(', ');
  execFileSync(
    'psql',
    [
      '-h',
      process.env.E2E_DB_HOST ?? 'localhost',
      '-p',
      process.env.E2E_DB_PORT ?? '5433',
      '-U',
      process.env.E2E_DB_USER ?? 'vibe',
      '-d',
      process.env.E2E_DB_NAME ?? 'vibe_e2e',
      '-c',
      `TRUNCATE ${tableList} CASCADE;`,
    ],
    {
      stdio: 'pipe',
      env: { ...process.env, PGPASSWORD: process.env.E2E_DB_PASSWORD ?? 'vibe_e2e' },
    },
  );
}
