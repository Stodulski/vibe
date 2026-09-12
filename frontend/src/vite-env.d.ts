/// <reference types="vite/client" />
/// <reference types="vite-plugin-pwa/client" />

interface ViteTypeOptions {
  strictImportMetaEnv: unknown;
}

interface ImportMetaEnv {
  VITE_API_URL?: string;
  VITE_APP_URL?: string;
  VITE_LANDING_URL?: string;
  VITE_SENTRY_DSN?: string;
  VITE_MP_APP_ID?: string;
  VITE_TURNSTILE_SITE_KEY?: string;
  VITE_GOOGLE_CLIENT_ID?: string;
}

/**
 * The commit that produced this build, or `'dev'` outside Vercel — see
 * `vite.config.ts`'s `define`. Read by `src/shared/lib/sentry.ts` as the
 * Sentry `release`.
 */
declare const APP_RELEASE: string;
