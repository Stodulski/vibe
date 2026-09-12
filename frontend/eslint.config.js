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
  // Dependencies flow one way: pages -> features -> shared. A feature never
  // imports from pages — see 06-auth-shared-tooling.md A2 and
  // 02-bookings-clients.md M4.
  {
    files: ['src/features/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': [
        'error',
        {
          patterns: [
            {
              group: ['@/pages/**'],
              message: 'A feature must not import from pages. Dependencies flow pages -> features -> shared.',
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
              ],
              message: "Import another feature's public API from its barrel, not its internals.",
            },
          ],
        },
      ],
    },
  },
  // Pages reach a feature only through its public API (the barrel) too —
  // see V-barrels-pages.md. Deep-importing `@/features/<X>/...` from a page
  // reintroduces exactly the coupling the per-feature blocks above prevent
  // between features.
  {
    files: ['src/pages/**/*.{ts,tsx}'],
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
              message: "Import a feature's public API from its barrel (e.g. '@/features/bookings'), not its internals.",
            },
          ],
        },
      ],
    },
  },
]);
