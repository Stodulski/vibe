import { z } from 'zod';

/**
 * Schema for every `VITE_*` variable actually read anywhere in `src/`
 * (`rg -n 'import.meta.env' src`). Every read site imports `env` from this
 * module (enforced by the `no-restricted-syntax` rule in eslint.config.js);
 * `VITE_APP_URL` is required so a deploy missing it fails the build instead
 * of silently falling back to `window.location.origin` in production.
 */
export const envSchema = z.object({
  VITE_API_URL: z.string().min(1).default('/api/v1'),
  VITE_APP_URL: z.url(),
  // Defaults to production so local dev needs no `.env` entry; only a
  // preview of the landing's MP-fees JSON contract needs to override it.
  VITE_LANDING_URL: z.url().default('https://vibe.com.ar'),
  VITE_SENTRY_DSN: z.string().optional(),
  VITE_MP_APP_ID: z.string().optional(),
  // Enables the Cloudflare Turnstile challenge on register/login/forgot-password
  // when set. Left unset, `TurnstileField` renders nothing and the three
  // forms behave exactly as they did before this field existed.
  VITE_TURNSTILE_SITE_KEY: z.string().optional(),
  // Enables "Continuar con Google" on login/register when set. Left unset,
  // `GoogleSignInSection` renders nothing and neither form changes behavior.
  VITE_GOOGLE_CLIENT_ID: z.string().optional(),
});

export type Env = z.infer<typeof envSchema>;

/** Parses and validates a raw `ImportMetaEnv`-shaped object, throwing a clear error on failure. */
export function parseEnv(source: ImportMetaEnv): Env {
  const result = envSchema.safeParse(source);
  if (!result.success) {
    const variables = [...new Set(result.error.issues.map((issue) => issue.path.join('.')))];
    throw new Error(`Invalid environment variables: ${variables.join(', ')}`);
  }
  return result.data;
}

/** Validated, typed environment. Import this instead of reading `import.meta.env` directly. */
export const env = parseEnv(import.meta.env);
