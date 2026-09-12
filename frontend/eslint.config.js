import js from '@eslint/js';
import globals from 'globals';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import tseslint from 'typescript-eslint';
import { defineConfig, globalIgnores } from 'eslint/config';

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.strictTypeChecked,
      tseslint.configs.stylisticTypeChecked,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      ecmaVersion: 2020,
      globals: globals.browser,
      parserOptions: {
        projectService: true,
        tsconfigRootDir: import.meta.dirname,
      },
    },
    rules: {
      // `reactHooks.configs.flat.recommended` ships this as 'warn', so a lying
      // dependency array passes CI silently. Zero violations exist in `src` as
      // of this change (`pnpm exec eslint src 2>&1 | rg exhaustive-deps`), so
      // raising it is a no-op today and a real gate going forward.
      'react-hooks/exhaustive-deps': 'error',
      // Pulling a key out of a payload so the rest object omits it is a
      // deliberate discard, not an unused variable. Each of those sites used to
      // carry a bare `void key;` to stay quiet — an idiom typescript-eslint
      // flags as meaningless, whose autofix strips the `void` and leaves an
      // unused expression behind. Naming the intent in one rule beats a
      // discard statement in every function that destructures.
      '@typescript-eslint/no-unused-vars': [
        'error',
        {
          ignoreRestSiblings: true,
          argsIgnorePattern: '^_',
          varsIgnorePattern: '^_',
          caughtErrorsIgnorePattern: '^_',
        },
      ],
      'max-lines': ['error', { max: 300, skipBlankLines: true, skipComments: true }],
      'max-lines-per-function': ['error', { max: 60, skipBlankLines: true, skipComments: true, IIFEs: true }],
      '@typescript-eslint/naming-convention': [
        'error',
        {
          selector: 'default',
          format: ['camelCase'],
          leadingUnderscore: 'allow',
          trailingUnderscore: 'allow',
        },
        // destructuring from the (snake_case) Go API payload keeps the original key name
        { selector: 'variable', modifiers: ['destructured'], format: null },
        {
          selector: 'variable',
          format: ['camelCase', 'PascalCase', 'UPPER_CASE'],
          leadingUnderscore: 'allow',
        },
        { selector: 'function', format: ['camelCase', 'PascalCase'] },
        { selector: 'parameter', modifiers: ['destructured'], format: null },
        // PascalCase allowed for component/icon props (e.g. `({ Icon }: Props)`)
        { selector: 'parameter', format: ['camelCase', 'PascalCase'], leadingUnderscore: 'allow' },
        { selector: 'typeLike', format: ['PascalCase'] },
        { selector: 'enumMember', format: ['PascalCase', 'UPPER_CASE'] },
        { selector: 'import', format: ['camelCase', 'PascalCase'] },
        // API payloads (snake_case), i18n keys, and mocked component references
        {
          selector: ['objectLiteralProperty', 'typeProperty', 'objectLiteralMethod'],
          format: null,
        },
      ],
    },
  },
  // The only production `console` call today is the `import.meta.env.DEV`-gated
  // `console.error` in apiParse.ts; this keeps a stray `console.log` from
  // sneaking back into shipped app code without a lint failure. Scoped to
  // `src/` — `e2e/`'s Playwright setup scripts are Node CLI tooling, where
  // `console.log` is the normal way to report progress.
  {
    files: ['src/**/*.{ts,tsx}'],
    rules: {
      'no-console': ['error', { allow: ['error', 'warn'] }],
    },
  },
  // jsx-a11y catches accessibility mistakes (missing alt text, a click
  // handler with no keyboard equivalent, an interactive role with no
  // matching semantics) at lint time rather than in a screen-reader pass —
  // see 06-auth-shared-tooling.md M8. Scoped to `src/**/*.tsx`: nothing
  // outside `src` renders JSX, and config/tooling `.tsx` fixtures (test
  // mocks, etc.) still fall under this since they live under `src` too.
  {
    files: ['src/**/*.tsx'],
    extends: [jsxA11y.flatConfigs.recommended],
  },
  // Dependencies flow one way: app -> features -> shared. Pages live inside
  // the feature that owns them (there is no top-level `src/pages` any more —
  // see ARQ-01), so the boundary a feature must not cross is `src/app`: the
  // router, the layouts and the providers compose features, never the other
  // way round — see 06-auth-shared-tooling.md A2 and 02-bookings-clients.md M4.
  {
    files: ['src/features/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: ['@/app/**'],
              message: 'A feature must not import from app. Dependencies flow app -> features -> shared.',
            },
          ],
        },
      ],
    },
  },
  // The other end of the same arrow: `src/shared` is domain-agnostic. It must
  // not reach into a feature or into app — see ARQ-02.
  {
    files: ['src/shared/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: ['@/features/**', '@/app/**'],
              message: 'shared is domain-agnostic: it must not import from features or app.',
            },
          ],
        },
      ],
    },
  },
  // Each feature exposes its public API from its own `index.ts`. Nothing
  // outside a feature imports its internals directly — see
  // 06-auth-shared-tooling.md A3 and 04-public-booking.md M10.
  {
    files: ['src/features/bookings/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: [
                '@/features/clients/**',
                '@/features/complex/**',
                '@/features/courts/**',
                '@/features/dashboard/**',
                '@/features/public-booking/**',
                '@/features/auth/**',
                '@/features/admin/**',
                '@/features/onboarding/**',
              ],
              message:
                "Import another feature's public API from its barrel (e.g. '@/features/clients'), not its internals.",
            },
          ],
        },
      ],
    },
  },
  {
    files: ['src/features/clients/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: [
                '@/features/bookings/**',
                '@/features/complex/**',
                '@/features/courts/**',
                '@/features/dashboard/**',
                '@/features/public-booking/**',
                '@/features/auth/**',
                '@/features/admin/**',
                '@/features/onboarding/**',
              ],
              message:
                "Import another feature's public API from its barrel (e.g. '@/features/bookings'), not its internals.",
            },
          ],
        },
      ],
    },
  },
  {
    files: ['src/features/complex/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: [
                '@/features/bookings/**',
                '@/features/clients/**',
                '@/features/courts/**',
                '@/features/dashboard/**',
                '@/features/public-booking/**',
                '@/features/auth/**',
                '@/features/admin/**',
                '@/features/onboarding/**',
              ],
              message:
                "Import another feature's public API from its barrel (e.g. '@/features/courts'), not its internals.",
            },
          ],
        },
      ],
    },
  },
  {
    files: ['src/features/courts/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: [
                '@/features/bookings/**',
                '@/features/clients/**',
                '@/features/complex/**',
                '@/features/dashboard/**',
                '@/features/public-booking/**',
                '@/features/auth/**',
                '@/features/admin/**',
                '@/features/onboarding/**',
              ],
              message:
                "Import another feature's public API from its barrel (e.g. '@/features/complex'), not its internals.",
            },
          ],
        },
      ],
    },
  },
  {
    files: ['src/features/dashboard/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: [
                '@/features/bookings/**',
                '@/features/clients/**',
                '@/features/complex/**',
                '@/features/courts/**',
                '@/features/public-booking/**',
                '@/features/auth/**',
                '@/features/admin/**',
                '@/features/onboarding/**',
              ],
              message:
                "Import another feature's public API from its barrel (e.g. '@/features/bookings'), not its internals.",
            },
          ],
        },
      ],
    },
  },
  {
    files: ['src/features/public-booking/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: [
                '@/features/bookings/**',
                '@/features/clients/**',
                '@/features/complex/**',
                '@/features/courts/**',
                '@/features/dashboard/**',
                '@/features/auth/**',
                '@/features/admin/**',
                '@/features/onboarding/**',
              ],
              message:
                "Import another feature's public API from its barrel (e.g. '@/features/complex'), not its internals.",
            },
          ],
        },
      ],
    },
  },
  {
    files: ['src/features/auth/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: [
                '@/features/bookings/**',
                '@/features/clients/**',
                '@/features/complex/**',
                '@/features/courts/**',
                '@/features/dashboard/**',
                '@/features/public-booking/**',
                '@/features/admin/**',
                '@/features/onboarding/**',
              ],
              message: "Import another feature's public API from its barrel, not its internals.",
            },
          ],
        },
      ],
    },
  },
  {
    files: ['src/features/admin/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: [
                '@/features/bookings/**',
                '@/features/clients/**',
                '@/features/complex/**',
                '@/features/courts/**',
                '@/features/dashboard/**',
                '@/features/public-booking/**',
                '@/features/auth/**',
                '@/features/onboarding/**',
              ],
              message: "Import another feature's public API from its barrel, not its internals.",
            },
          ],
        },
      ],
    },
  },
  {
    files: ['src/features/onboarding/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: [
                '@/features/bookings/**',
                '@/features/clients/**',
                '@/features/complex/**',
                '@/features/courts/**',
                '@/features/dashboard/**',
                '@/features/public-booking/**',
                '@/features/auth/**',
                '@/features/admin/**',
              ],
              message: "Import another feature's public API from its barrel, not its internals.",
            },
          ],
        },
      ],
    },
  },
  // Both selectors below live in one no-restricted-syntax block (same rule
  // key, same 'error' severity) rather than two separate config objects:
  // flat config lets the LAST config object matching a file win a given
  // rule key OUTRIGHT instead of merging it with an earlier match, and the
  // per-feature/pages no-restricted-imports blocks above already claim that
  // key for most of `src` — a different rule key here keeps both checks
  // additive everywhere instead of one silently discarding the other.
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/shared/lib/env.ts', '**/*.test.{ts,tsx}'],
    rules: {
      'no-restricted-syntax': [
        'error',
        // CI-03/BLD-04: raw `import.meta.env.VITE_*` reads are only allowed
        // inside src/shared/lib/env.ts, where the Zod schema in envSchema
        // validates them once at startup — every other read site imports
        // the typed `env` export instead. `MODE`/`DEV`/`PROD`/`BASE_URL`/
        // `SSR` are Vite's own built-ins, not app config, so they are
        // exempt. Tests are exempt too: several (sentry.test.ts,
        // env.test.ts) deliberately stub/parse raw env values to exercise
        // env.ts and initSentry() in isolation.
        {
          selector:
            "MemberExpression[object.object.type='MetaProperty'][object.property.name='env'][property.name=/^VITE_/]",
          message: "Import `env` from '@/shared/lib/env' instead of reading `import.meta.env.VITE_*` directly.",
        },
        // CI-03: `import { X } from 'lucide-react'` (and date-fns) named
        // imports are the correct, tree-shakeable form — only the
        // namespace import pulls in the whole package. 167 files already
        // use named imports from lucide-react, so unlike a blanket barrel
        // ban this is clean today (`rg -n "import \* as .* from
        // '(lucide-react|date-fns)'" src` finds nothing) and can sit at
        // 'error'.
        {
          selector: 'ImportNamespaceSpecifier[parent.source.value=/^(lucide-react|date-fns)$/]',
          message: "Import only the named icons/functions you use, not the whole module with 'import * as'.",
        },
      ],
    },
  },
  // CI-03: raw `fetch` is only allowed inside src/shared/lib (the ky client
  // it backs) plus the two sites that deliberately bypass ky because ky's
  // cookie/CSRF behavior is wrong for them: mpFees.api.ts calls a public
  // static file on a different origin (the landing), and upload.api.ts PUTs
  // straight to a presigned R2 URL.
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: [
      'src/shared/lib/**',
      'src/features/complex/api/mpFees.api.ts',
      'src/features/complex/api/upload.api.ts',
      '**/*.test.{ts,tsx}',
    ],
    rules: {
      'no-restricted-globals': ['error', { name: 'fetch', message: "Usá el cliente ky de '@/shared/lib/ky'." }],
    },
  },
]);
