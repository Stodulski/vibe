/**
 * Tests for the drift checker.
 *
 * The whole value of that script is failing when two copies disagree, and CI
 * only ever runs it against a tree where they agree — so every run passes and a
 * regex that quietly matches the wrong text would pass too. These build small
 * fixture trees instead, and assert on the errors it reports as much as on its
 * exit code: a check that fails without naming the file is barely better than
 * one that does not fail.
 *
 * Assertions read the parsed `::error` annotations, never the raw output. The
 * script prints a summary table naming all four files before it reports any
 * mismatch, so searching the whole output for a path matches that table and
 * proves nothing — a test that passes for the wrong reason is the same defect
 * as the one it is meant to catch.
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

/**
 * The `::error file=path::message` annotations the script reports, parsed.
 *
 * Split rather than matched, because a message can run over several lines —
 * the "reworded past its pattern" one carries the pattern and the instruction
 * on their own lines — and a `$` anchor under the `m` flag would cut it at the
 * first. The trailing summary line is separated from the last message by a
 * blank line, which is where each message ends.
 */
const annotations = (stderr) =>
  stderr
    .split(/^(?=::error file=)/m)
    .filter((chunk) => chunk.startsWith('::error file='))
    .map((chunk) => {
      const [, file, message] = /^::error file=([^:]+)::([\s\S]*)$/.exec(chunk.split('\n\n')[0].trimEnd());
      return { file, message };
    });

/** Writes a fixture tree and runs the checker against it. */
function run(files) {
  const root = mkdtempSync(join(tmpdir(), 'fee-'));
  try {
    for (const [path, body] of Object.entries(files)) {
      mkdirSync(join(root, dirname(path)), { recursive: true });
      writeFileSync(join(root, path), body);
    }
    const opts = { env: { ...process.env, SERVICE_FEE_REPO_ROOT: root }, encoding: 'utf8' };
    try {
      return { code: 0, stdout: execFileSync('node', [SCRIPT], opts), errors: [] };
    } catch (err) {
      const stderr = err.stderr ?? '';
      return { code: err.status, stdout: err.stdout ?? '', errors: annotations(stderr) };
    }
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

/** The single error reported for `file`, asserting there is exactly one. */
function only(errors, file) {
  const mine = errors.filter((e) => e.file.endsWith(file));
  assert.equal(mine.length, 1, `expected exactly one error for ${file}, got ${JSON.stringify(errors, null, 1)}`);
  return mine[0].message;
}

test('passes when all four agree', () => {
  const { code, stdout } = run(tree());
  assert.equal(code, 0, stdout);
  assert.match(stdout, /all 4 agree: 7% with a \$1\.000 floor\./);
});

test('catches a rate that drifted on the landing', () => {
  const { code, errors } = run(tree({ landingRate: '0.06' }));
  assert.equal(code, 1);
  assert.equal(errors.length, 1);
  assert.match(only(errors, 'landing/src/data/precio.ts'), /rate is 6% here but 7%/);
});

// The floor moving in the backend must be reported against each follower
// separately. Asserting only that the output mentions the three files would
// pass on the summary table alone, which names them however the run went.
test('names every follower, one error each, when the backend floor moves alone', () => {
  const { code, errors } = run(tree({ goFloor: '150_000' }));
  assert.equal(code, 1);
  assert.equal(errors.length, 3, 'one per follower, and nothing else');
  for (const file of [
    'frontend/src/features/public-booking/components/booking-form/pricing.ts',
    'frontend/src/shared/i18n/es_AR/serviceFee.ts',
    'landing/src/data/precio.ts',
  ]) {
    assert.match(only(errors, file), /floor is \$1\.000 here but \$1\.500/);
  }
});

test('catches a follower whose floor drifts while the backend stays put', () => {
  const { code, errors } = run(tree({ feFloor: '150_000' }));
  assert.equal(code, 1);
  assert.equal(errors.length, 1);
  assert.match(only(errors, 'booking-form/pricing.ts'), /floor is \$1\.500 here but \$1\.000/);
});

test('catches the client-side estimate drifting from the backend formula', () => {
  const { code, errors } = run(tree({ feRate: '8' }));
  assert.equal(code, 1);
  assert.equal(errors.length, 1);
  assert.match(only(errors, 'booking-form/pricing.ts'), /rate is 8% here but 7%/);
});

test('catches the owner copy drifting', () => {
  const copy = `const a = 'Tus clientes pagan un cargo de servicio del 9% (mínimo $1.000) al reservar.';\n`;
  const { code, errors } = run(tree({ copy }));
  assert.equal(code, 1);
  assert.equal(errors.length, 1);
  assert.match(only(errors, 'i18n/es_AR/serviceFee.ts'), /rate is 9% here but 7%/);
});

// The case that made the first version of this script unsound: it read one
// match per file, so a second mention was free to drift.
test('catches a second mention in the same file that disagrees', () => {
  const copy =
    `const a = 'Tus clientes pagan un cargo de servicio del 7% (mínimo $1.000) al reservar.';\n` +
    `const b = 'Recordá: cargo de servicio del 9% (mínimo $1.000).';\n`;
  const { code, errors } = run(tree({ copy }));
  assert.equal(code, 1);
  assert.match(only(errors, 'i18n/es_AR/serviceFee.ts'), /states the rate in the owner copy 2 times and they disagree: 7, 9/);
});

test('fails loudly when a sentence is reworded past its pattern', () => {
  const copy = `const a = 'Cobramos 7 por ciento, con piso de mil pesos.';\n`;
  const { code, errors } = run(tree({ copy }));
  assert.equal(code, 1);
  const message = only(errors, 'i18n/es_AR/serviceFee.ts');
  assert.match(message, /could not read the rate in the owner copy/);
  assert.match(message, /do not delete the case/);
});

test('reads a floor written with or without a thousands separator', () => {
  assert.equal(run(tree()).code, 0);
  const plain = run(tree({ copy: `const a = 'cargo de servicio del 7% (mínimo $1000)';\n` }));
  assert.equal(plain.code, 0, 'the same amount written $1000 must read as $1.000');
});

// 0.07 * 100 is 7.000000000000001 in binary floating point; comparing that to
// the backend's integer 7 without rounding would report a mismatch that is not
// there — and the landing is the only site that states the rate as a fraction.
test('compares the fractional landing rate against the integer backend one without float noise', () => {
  const { code, errors, stdout } = run(tree({ landingRate: '0.07' }));
  assert.equal(code, 0, `${stdout}${JSON.stringify(errors)}`);
});

test('reports a missing file rather than passing on three of four', () => {
  const files = tree();
  delete files['landing/src/data/precio.ts'];
  const { code, errors } = run(files);
  assert.equal(code, 1);
  assert.match(only(errors, 'landing/src/data/precio.ts'), /ENOENT|no such file/);
});
