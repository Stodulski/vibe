/**
 * Tests for the drift checker.
 *
 * The whole value of that script is failing when two copies disagree, and CI
 * only ever runs it against a tree where they agree — so every run passes and a
 * regex that quietly matches the wrong text would pass too. These build small
 * fixture trees instead, and assert on what it says as much as on its exit code:
 * a check that fails without naming the file is barely better than one that
 * does not fail.
 *
 * Run: node --test .github/scripts/
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const SCRIPT = resolve(dirname(fileURLToPath(import.meta.url)), 'check-service-fee.mjs');

/** The four files as they read when everything agrees, at 7% with a $1.000 floor. */
function tree({ goRate = '7', goFloor = '100_000', feRate = '7', feFloor = '100_000', copy, landingRate = '0.07', landingFloor = '1000' } = {}) {
  return {
    'backend/internal/pricing/pricing.go':
      `package pricing\n\nconst feeRatePercent = ${goRate}\n\nconst feeMinCentavos = ${goFloor}\n`,
    'frontend/src/features/public-booking/components/booking-form/pricing.ts':
      `const serviceFee = slotInfo.serviceFee ?? Math.max(Math.round((mpAmount * ${feRate}) / 100), ${feFloor});\n`,
    'frontend/src/shared/i18n/es_AR/serviceFee.ts':
      copy ?? `const a = 'Tus clientes pagan un cargo de servicio del 7% (mínimo $1.000) al reservar.';\n`,
    'landing/src/data/precio.ts':
      `export const CARGO_SERVICIO = ${landingRate};\nexport const CARGO_MINIMO = ${landingFloor};\n`,
  };
}

/** Writes a fixture tree and runs the checker against it. */
function run(files) {
  const root = mkdtempSync(join(tmpdir(), 'fee-'));
  try {
    for (const [path, body] of Object.entries(files)) {
      mkdirSync(join(root, dirname(path)), { recursive: true });
      writeFileSync(join(root, path), body);
    }
    try {
      const stdout = execFileSync('node', [SCRIPT], { env: { ...process.env, SERVICE_FEE_REPO_ROOT: root }, encoding: 'utf8' });
      return { code: 0, output: stdout };
    } catch (err) {
      return { code: err.status, output: `${err.stdout ?? ''}${err.stderr ?? ''}` };
    }
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

test('passes when all four agree', () => {
  const { code, output } = run(tree());
  assert.equal(code, 0, output);
  assert.match(output, /all 4 agree: 7% with a \$1\.000 floor\./);
});

test('catches a rate that drifted on the landing', () => {
  const { code, output } = run(tree({ landingRate: '0.06' }));
  assert.equal(code, 1);
  assert.match(output, /landing\/src\/data\/precio\.ts/);
  assert.match(output, /rate is 6% here but 7%/);
});

test('names every follower when the backend floor moves alone', () => {
  const { code, output } = run(tree({ goFloor: '150_000' }));
  assert.equal(code, 1);
  for (const file of ['booking-form/pricing.ts', 'i18n/es_AR/serviceFee.ts', 'landing/src/data/precio.ts']) {
    assert.match(output, new RegExp(file.replace(/[.\/]/g, '\\$&')));
  }
});

test('catches the client-side estimate drifting from the backend formula', () => {
  const { code, output } = run(tree({ feRate: '8' }));
  assert.equal(code, 1);
  assert.match(output, /booking-form\/pricing\.ts/);
  assert.match(output, /rate is 8% here but 7%/);
});

// The case that made the first version of this script unsound: it read one
// match per file, so a second mention was free to drift.
test('catches a second mention in the same file that disagrees', () => {
  const copy =
    `const a = 'Tus clientes pagan un cargo de servicio del 7% (mínimo $1.000) al reservar.';\n` +
    `const b = 'Recordá: cargo de servicio del 9% (mínimo $1.000).';\n`;
  const { code, output } = run(tree({ copy }));
  assert.equal(code, 1);
  assert.match(output, /states the rate in the owner copy 2 times and they disagree: 7, 9/);
});

test('fails loudly when a sentence is reworded past its pattern', () => {
  const copy = `const a = 'Cobramos 7 por ciento, con piso de mil pesos.';\n`;
  const { code, output } = run(tree({ copy }));
  assert.equal(code, 1);
  assert.match(output, /could not read the rate in the owner copy/);
  assert.match(output, /do not delete the case/);
});

test('reads a floor written with or without a thousands separator', () => {
  const withDot = run(tree()).code;
  const plain = run(tree({ copy: `const a = 'cargo de servicio del 7% (mínimo $1000)';\n` })).code;
  assert.equal(withDot, 0);
  assert.equal(plain, 0, 'the same amount written $1000 must read as $1.000');
});

// 0.07 * 100 is 7.000000000000001 in binary floating point; a naive comparison
// against the backend's integer 7 would report a mismatch that does not exist.
test('compares a fractional rate against an integer one without float noise', () => {
  const { code, output } = run(tree({ landingRate: '0.07' }));
  assert.equal(code, 0, output);
});

test('reports a missing file rather than passing on three of four', () => {
  const files = tree();
  delete files['landing/src/data/precio.ts'];
  const { code, output } = run(files);
  assert.equal(code, 1);
  assert.match(output, /landing\/src\/data\/precio\.ts/);
});
