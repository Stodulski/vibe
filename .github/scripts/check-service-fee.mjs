#!/usr/bin/env node
/**
 * The service fee is one commercial fact written in four places, in three
 * languages, none of which can import the others: the Go that charges it, the
 * client-side fallback that estimates it before the backend has quoted, the
 * Spanish the owner reads while connecting payments, and the Spanish a visitor
 * reads on the public site.
 *
 * It is a flat amount per online payment, so there is exactly one number to
 * keep in step. Four hand-kept copies of a statement about money is one
 * somebody updates without updating the other three — and the failure is not a
 * crash, it is a landing page advertising a price the checkout does not charge.
 * This compares them against the backend and fails naming both values.
 *
 * The terms page (landing/src/pages/terminos.astro) is the fifth place the fee
 * is written, and it is deliberately not a copy: it imports the landing
 * constant. So instead of comparing a value it is held to that arrangement —
 * it must keep importing the constant, and no sentence about the fee may carry a
 * hard-coded amount or percentage, which is how it drifted before.
 *
 * It reads the files as text on purpose. Importing them would mean a Go
 * toolchain, a bundler and an Astro resolver in one job to learn one number.
 *
 * A pattern that stops matching fails the same way a mismatch does: a reworded
 * sentence is exactly when this check has to speak up, not quietly pass.
 */
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

// Overridable so the test can point the same code at a fixture tree. Nothing
// else sets it, and CI runs the script with it unset.
const REPO = process.env.SERVICE_FEE_REPO_ROOT ?? join(dirname(fileURLToPath(import.meta.url)), '..', '..');

/**
 * Reads one capture group from EVERY place the pattern matches, and returns it
 * only when they all say the same thing.
 *
 * Reading the first match would be the silent pass this script exists to
 * prevent: a file is free to state the fee more than once — the owner copy
 * already splits it across two sentences and rejoins them — and a second
 * mention that drifts while the first stays right would sail through.
 */
function capture(source, file, pattern, what) {
  const found = [...source.matchAll(new RegExp(pattern, pattern.flags.replace('g', '') + 'g'))].map((m) => m[1]);

  if (found.length === 0) {
    throw new Error(
      `could not read ${what} in ${file}\n` +
        `      pattern: ${pattern}\n` +
        `      The value probably moved or the sentence was reworded. Re-point this check at it — ` +
        `do not delete the case.`,
    );
  }

  const distinct = [...new Set(found)];
  if (distinct.length > 1) {
    throw new Error(
      `${file} states ${what} ${found.length} times and they disagree: ${distinct.join(', ')}\n` +
        `      One file contradicting itself is the same bug as two files contradicting each other.`,
    );
  }

  return distinct[0];
}

/** Centavos, from an amount written in centavos ("100_000" or "100000"). */
const fromCentavos = (s) => Number(s.replaceAll('_', ''));
/** Centavos, from an amount written in pesos ("1.000" or "1000"). */
const toCentavos = (s) => Number(s.replace(/\./g, '')) * 100;

const SITES = [
  {
    label: 'backend — what is actually charged',
    file: 'backend/internal/pricing/pricing.go',
    read: (s, f) => ({
      amountCentavos: fromCentavos(capture(s, f, /^const serviceFeeCentavos = ([\d_]+)$/m, 'serviceFeeCentavos')),
    }),
  },
  {
    label: 'booking — client-side estimate before the backend quotes',
    file: 'packages/booking/src/components/booking-form/pricing.ts',
    read: (s, f) => ({
      amountCentavos: fromCentavos(
        capture(s, f, /^const SERVICE_FEE_FALLBACK_CENTAVOS = ([\d_]+);$/m, 'SERVICE_FEE_FALLBACK_CENTAVOS'),
      ),
    }),
  },
  {
    label: 'frontend — what the owner reads when connecting payments',
    file: 'frontend/src/shared/i18n/es_AR/serviceFee.ts',
    read: (s, f) => ({
      amountCentavos: toCentavos(capture(s, f, /cargo de servicio fijo de \$([\d.]+)/, 'the amount in the owner copy')),
    }),
  },
  {
    label: 'landing — what a visitor is promised',
    file: 'landing/src/data/precio.ts',
    read: (s, f) => ({
      amountCentavos: toCentavos(capture(s, f, /^export const CARGO_SERVICIO = (\d+);$/m, 'CARGO_SERVICIO')),
    }),
  },
];

/**
 * The terms page states the fee by importing the landing constant, so there is
 * no value to compare — only the arrangement to keep. Throws, like `capture`,
 * when the arrangement is gone or the page has started hard-coding the fee.
 */
const TERMS_FILE = 'landing/src/pages/terminos.astro';
function checkTerms(source, file) {
  if (!/import\s*\{[^}]*\bCARGO_SERVICIO\b[^}]*\}\s*from\s*'\.\.\/data\/precio(?:\.ts)?'/.test(source)) {
    throw new Error(
      `${file} no longer imports CARGO_SERVICIO from ../data/precio\n` +
        `      The terms read the amount from there so they cannot drift. If they moved to another way of ` +
        `reading it, re-point this check — do not delete it.`,
    );
  }
  if (!/\{pesos\(CARGO_SERVICIO\)\}/.test(source)) {
    throw new Error(
      `${file} imports CARGO_SERVICIO but no longer renders it with {pesos(CARGO_SERVICIO)}\n` +
        `      The sentence that states the fee amount probably lost its expression. Re-point this check at it.`,
    );
  }

  const hardCoded = source
    .split('\n')
    .map((line, i) => ({ line: line.trim(), n: i + 1 }))
    .filter(({ line }) => /cargo de servicio/i.test(line) && /\$\s?\d|\d\s*%/.test(line));
  if (hardCoded.length > 0) {
    throw new Error(
      `${file} writes the service fee amount or a percentage by hand:\n` +
        hardCoded.map(({ line, n }) => `      line ${n}: ${line}`).join('\n') +
        `\n      Render it with {pesos(CARGO_SERVICIO)} instead, so it cannot drift from the real fee.`,
    );
  }
}

const pesos = (centavos) => `$${(centavos / 100).toLocaleString('es-AR')}`;

let failures = 0;
const rows = [];

for (const site of SITES) {
  try {
    rows.push({ ...site, ...site.read(readFileSync(join(REPO, site.file), 'utf8'), site.file) });
  } catch (err) {
    console.error(`::error file=${site.file}::${err.message}`);
    failures += 1;
  }
}

if (failures > 0) process.exit(1);

try {
  checkTerms(readFileSync(join(REPO, TERMS_FILE), 'utf8'), TERMS_FILE);
} catch (err) {
  console.error(`::error file=${TERMS_FILE}::${err.message}`);
  failures += 1;
}

if (failures > 0) process.exit(1);

const [truth, ...rest] = rows;

console.log('service fee, as each package states it:\n');
for (const r of rows) {
  console.log(`  ${pesos(r.amountCentavos).padEnd(8)}  ${r.file}`);
  console.log(`            ${r.label}`);
}
console.log(`  imports   ${TERMS_FILE}`);
console.log('            landing — the terms read the amount from precio.ts');
console.log();

for (const r of rest) {
  if (r.amountCentavos !== truth.amountCentavos) {
    console.error(
      `::error file=${r.file}::service fee is ${pesos(r.amountCentavos)} here but ${pesos(truth.amountCentavos)} in ` +
        `${truth.file}, which is what a client is actually charged. Whichever is wrong, they cannot ship apart.`,
    );
    failures += 1;
  }
}

if (failures > 0) {
  console.error(`\n${failures} mismatch(es). The fee is one fact; it has ${SITES.length} copies and they must agree.`);
  process.exit(1);
}

console.log(`all ${SITES.length} agree: a flat ${pesos(truth.amountCentavos)} per online payment.`);
