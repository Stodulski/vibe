/**
 * Build-time gate for the variables a production bundle cannot run without.
 *
 * `import.meta.env` is substituted statically by `vite build`, so the schema
 * in `env.ts` (evaluated at module load) only ever runs in the browser: a
 * deploy that forgets a variable would build fine and then fail in every
 * user's tab. `vite.config.ts` calls this on `vite build` so the deploy fails
 * instead (BLD-04). Dev and e2e servers are not gated; `env.ts` falls back to
 * the page origin there.
 */
export const REQUIRED_BUILD_ENV = ['VITE_APP_URL'] as const;

export function assertBuildEnv(source: Record<string, string | undefined>): void {
  const missing = REQUIRED_BUILD_ENV.filter((name) => !source[name]?.trim());
  if (missing.length > 0) {
    throw new Error(
      `Missing required build environment variables: ${missing.join(', ')}. ` +
        'Set them in the deploy environment or a .env file (see .env.example).',
    );
  }
  for (const name of REQUIRED_BUILD_ENV) {
    if (!URL.canParse(source[name] ?? '')) {
      throw new Error(`${name} must be an absolute URL, got "${source[name] ?? ''}".`);
    }
  }
}
