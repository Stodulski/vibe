// @vitest-environment node
import { describe, expect, it } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// The package ships on its own. It must not import the app that mounts it: no
// `@/` alias (that alias belongs to the app), no router, and no path that
// reaches out of the package into the app. ESLint enforces the same rule
// (`no-restricted-imports`); this test keeps it true where ESLint is not run.

const SRC = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const FORBIDDEN = [/^@\//, /^react-router/, /(^|\/)frontend\//];
const SPECIFIER = /(?:from|import|vi\.mock|vi\.doMock)\s*\(?\s*['"]([^'"]+)['"]/g;

function sourceFiles(dir: string): string[] {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) return sourceFiles(full);
    return /\.(ts|tsx)$/.test(entry.name) ? [full] : [];
  });
}

describe('package boundaries', () => {
  it('imports nothing from the app, the router or outside the package', () => {
    const offenders: string[] = [];
    for (const file of sourceFiles(SRC)) {
      const text = fs.readFileSync(file, 'utf8');
      for (const match of text.matchAll(SPECIFIER)) {
        const specifier = match[1] ?? '';
        if (FORBIDDEN.some((pattern) => pattern.test(specifier))) {
          offenders.push(`${path.relative(SRC, file)} -> ${specifier}`);
        }
      }
    }
    expect(offenders).toEqual([]);
  });
});
