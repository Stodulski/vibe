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

const TERMS = 'landing/src/pages/terminos.astro';

/** The terms page as it reads when it takes the fee from the landing constant. */
const TERMS_OK =
  `---\nimport { CARGO_SERVICIO, pesos } from '../data/precio';\n---\n` +
  `<li>Se aplica un cargo de servicio fijo de {pesos(CARGO_SERVICIO)} ARS por cada pago online</li>\n`;

/** The five files as they read when everything agrees, at a flat $1.000. */
function tree({
  goAmount = '100_000',
  feAmount = '100_000',
  copy,
  landingAmount = '1000',
  terms = TERMS_OK,
} = {}) {
  return {
    'backend/internal/pricing/pricing.go': `package pricing\n\nconst serviceFeeCentavos = ${goAmount}\n`,
    'frontend/src/features/public-booking/components/booking-form/pricing.ts':
      `const SERVICE_FEE_FALLBACK_CENTAVOS = ${feAmount};\n`,
    'frontend/src/shared/i18n/es_AR/serviceFee.ts':
      copy ?? `const a = 'Tus clientes pagan un cargo de servicio fijo de $1.000 por reserva.';\n`,
    'landing/src/data/precio.ts': `export const CARGO_SERVICIO = ${landingAmount};\n`,
    [TERMS]: terms,
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

test('passes when all four agree and the terms read the constant', () => {
  const { code, stdout } = run(tree());
  assert.equal(code, 0, stdout);
  assert.match(stdout, /all 4 agree: a flat \$1\.000 per online payment\./);
});

test('catches an amount that drifted on the landing', () => {
  const { code, errors } = run(tree({ landingAmount: '1500' }));
  assert.equal(code, 1);
  assert.equal(errors.length, 1);
  assert.match(only(errors, 'landing/src/data/precio.ts'), /fee is \$1\.500 here but \$1\.000/);
});

// The amount moving in the backend must be reported against each follower
// separately. Asserting only that the output mentions the three files would
// pass on the summary table alone, which names them however the run went.
test('names every follower, one error each, when the backend amount moves alone', () => {
  const { code, errors } = run(tree({ goAmount: '150_000' }));
  assert.equal(code, 1);
  assert.equal(errors.length, 3, 'one per follower, and nothing else');
  for (const file of [
    'frontend/src/features/public-booking/components/booking-form/pricing.ts',
    'frontend/src/shared/i18n/es_AR/serviceFee.ts',
    'landing/src/data/precio.ts',
  ]) {
    assert.match(only(errors, file), /fee is \$1\.000 here but \$1\.500/);
  }
});

test('catches the client-side fallback drifting from the backend', () => {
  const { code, errors } = run(tree({ feAmount: '150_000' }));
  assert.equal(code, 1);
  assert.equal(errors.length, 1);
  assert.match(only(errors, 'booking-form/pricing.ts'), /fee is \$1\.500 here but \$1\.000/);
});

test('catches the owner copy drifting', () => {
  const copy = `const a = 'Tus clientes pagan un cargo de servicio fijo de $2.000 por reserva.';\n`;
  const { code, errors } = run(tree({ copy }));
  assert.equal(code, 1);
  assert.equal(errors.length, 1);
  assert.match(only(errors, 'i18n/es_AR/serviceFee.ts'), /fee is \$2\.000 here but \$1\.000/);
});

// The case that made the first version of this script unsound: it read one
// match per file, so a second mention was free to drift.
test('catches a second mention in the same file that disagrees', () => {
  const copy =
    `const a = 'Tus clientes pagan un cargo de servicio fijo de $1.000 por reserva.';\n` +
    `const b = 'Recordá: cargo de servicio fijo de $2.000.';\n`;
  const { code, errors } = run(tree({ copy }));
  assert.equal(code, 1);
  assert.match(
    only(errors, 'i18n/es_AR/serviceFee.ts'),
    /states the amount in the owner copy 2 times and they disagree: 1\.000, 2\.000/,
  );
});

test('fails loudly when a sentence is reworded past its pattern', () => {
  const copy = `const a = 'Cobramos mil pesos por reserva online.';\n`;
  const { code, errors } = run(tree({ copy }));
  assert.equal(code, 1);
  const message = only(errors, 'i18n/es_AR/serviceFee.ts');
  assert.match(message, /could not read the amount in the owner copy/);
  assert.match(message, /do not delete the case/);
});

test('reads an amount written with or without a thousands separator', () => {
  assert.equal(run(tree()).code, 0);
  const plain = run(tree({ copy: `const a = 'cargo de servicio fijo de $1000 por reserva';\n` }));
  assert.equal(plain.code, 0, 'the same amount written $1000 must read as $1.000');
});

test('fails when the backend constant is renamed past its pattern', () => {
  const files = tree();
  files['backend/internal/pricing/pricing.go'] = 'package pricing\n\nconst flatFee = 100_000\n';
  const { code, errors } = run(files);
  assert.equal(code, 1);
  assert.match(only(errors, 'backend/internal/pricing/pricing.go'), /could not read serviceFeeCentavos/);
});

test('reports a missing file rather than passing on three of four', () => {
  const files = tree();
  delete files['landing/src/data/precio.ts'];
  const { code, errors } = run(files);
  assert.equal(code, 1);
  assert.match(only(errors, 'landing/src/data/precio.ts'), /ENOENT|no such file/);
});

test('reports a missing terms page rather than skipping it', () => {
  const files = tree();
  delete files[TERMS];
  const { code, errors } = run(files);
  assert.equal(code, 1);
  assert.match(only(errors, TERMS), /ENOENT|no such file/);
});

// The terms page used to hard-code "7% (mínimo $1.000 ARS)" and nothing noticed
// when the real fee moved.
test('fails when the terms hard-code an amount instead of reading the constant', () => {
  const terms = TERMS_OK + `<li>Se aplica un cargo de servicio de $1.000 al cliente</li>\n`;
  const { code, errors } = run(tree({ terms }));
  assert.equal(code, 1);
  assert.match(only(errors, TERMS), /writes the service fee amount or a percentage by hand/);
});

test('fails when the terms describe the fee as a percentage', () => {
  const terms = TERMS_OK + `<li>El cargo de servicio es del 7% de la seña</li>\n`;
  const { code, errors } = run(tree({ terms }));
  assert.equal(code, 1);
  assert.match(only(errors, TERMS), /writes the service fee amount or a percentage by hand/);
});

test('fails when the terms stop importing the constant', () => {
  const terms = `<li>Se aplica un cargo de servicio fijo al cliente</li>\n`;
  const { code, errors } = run(tree({ terms }));
  assert.equal(code, 1);
  assert.match(only(errors, TERMS), /no longer imports CARGO_SERVICIO/);
});

test('fails when the terms import the constant but stop rendering it', () => {
  const terms = `---\nimport { CARGO_SERVICIO } from '../data/precio';\n---\n<li>Se aplica un cargo de servicio fijo</li>\n`;
  const { code, errors } = run(tree({ terms }));
  assert.equal(code, 1);
  assert.match(only(errors, TERMS), /no longer renders it/);
});
