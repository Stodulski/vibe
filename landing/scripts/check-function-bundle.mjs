// Fails the build when the SSR function bundle imports build-time tooling.
//
// @astrojs/vercel 11.0.13 lets its build-time edge-middleware builder, which
// imports rolldown at top level, leak into the function entry. Vercel does not
// deploy rolldown's native binding, so the function crashed on its first
// request. astro.config.mjs stubs that import; this check catches the next
// adapter release that reaches build tooling another way.
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';

const roots = ['.vercel/output/_functions', '.vercel/output/functions'];
const forbidden = [
  /from\s*["']rolldown["']/,
  /require\(\s*["']rolldown["']\s*\)/,
  /@rolldown\/binding/,
  /from\s*["']vite["']/,
  /from\s*["']esbuild["']/,
];

function* files(dir) {
  let entries;
  try {
    entries = readdirSync(dir);
  } catch {
    return;
  }
  for (const name of entries) {
    const path = join(dir, name);
    const stat = statSync(path);
    if (stat.isDirectory()) {
      if (name !== 'node_modules') yield* files(path);
    } else if (/\.(c|m)?js$/.test(name)) {
      yield path;
    }
  }
}

const hits = [];
let scanned = 0;
for (const root of roots) {
  for (const file of files(root)) {
    scanned += 1;
    const source = readFileSync(file, 'utf8');
    for (const pattern of forbidden) {
      if (pattern.test(source)) hits.push(`${file}: ${pattern}`);
    }
  }
}

if (scanned === 0) {
  console.error('check-function-bundle: no function output found under .vercel/output');
  process.exit(1);
}
if (hits.length > 0) {
  console.error('check-function-bundle: the SSR function imports build tooling:');
  for (const hit of hits) console.error(`  ${hit}`);
  process.exit(1);
}
console.log(`check-function-bundle: ${scanned} files clean`);
