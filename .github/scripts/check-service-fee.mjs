#!/usr/bin/env node
/**
 * The service fee is one commercial fact written in four places, in three
 * languages, none of which can import the others: the Go that charges it, the
 * client-side fallback that estimates it before the backend has quoted, the
 * Spanish the owner reads while connecting payments, and the Spanish a visitor
 * reads on the public site.
 *
 * Four hand-kept copies of a statement about money is one somebody updates
 * without updating the other three — and the failure is not a crash, it is a
 * landing page advertising a price the checkout does not charge. This compares
 * them against the backend and fails naming both values.
 *
 * It reads the files as text on purpose. Importing them would mean a Go
 * toolchain, a bundler and an Astro resolver in one job to learn two numbers.
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

/** Percent as an integer (7), from a rate written as a fraction (0.07). */
const fromFraction = (s) => Math.round(Number(s) * 100 * 1e6) / 1e6;
/** Centavos, from an amount written in pesos ("1.000" or "1000"). */
const toCentavos = (s) => Number(s.replace(/\./g, '')) * 100;

const SITES = [
  {
    label: 'backend — what is actually charged',
    file: 'backend/internal/pricing/pricing.go',
    read: (s, f) => ({
      ratePercent: Number(capture(s, f, /^const feeRatePercent = (\d+)$/m, 'feeRatePercent')),
      floorCentavos: Number(capture(s, f, /^const feeMinCentavos = ([\d_]+)$/m, 'feeMinCentavos').replaceAll('_', '')),
    }),
  },
  {
    label: 'frontend — client-side estimate before the backend quotes',
    file: 'frontend/src/features/public-booking/components/booking-form/pricing.ts',
    read: (s, f) => ({
      ratePercent: Number(
        capture(s, f, /Math\.max\(Math\.round\(\(mpAmount \* (\d+)\) \/ 100\), [\d_]+\)/, 'the fallback rate'),
      ),
      floorCentavos: Number(
        capture(
          s,
          f,
          /Math\.max\(Math\.round\(\(mpAmount \* \d+\) \/ 100\), ([\d_]+)\)/,
          'the fallback floor',
        ).replaceAll('_', ''),
      ),
    }),
  },
  {
    label: 'frontend — what the owner reads when connecting payments',
    file: 'frontend/src/shared/i18n/es_AR/serviceFee.ts',
    read: (s, f) => ({
      ratePercent: Number(capture(s, f, /cargo de servicio del (\d+)%/, 'the rate in the owner copy')),
      floorCentavos: toCentavos(capture(s, f, /mínimo \$([\d.]+)/, 'the floor in the owner copy')),
    }),
  },
  {
    label: 'landing — what a visitor is promised',
    file: 'landing/src/data/precio.ts',
    read: (s, f) => ({
      ratePercent: fromFraction(capture(s, f, /export const CARGO_SERVICIO = ([\d.]+);/, 'CARGO_SERVICIO')),
      floorCentavos: toCentavos(capture(s, f, /export const CARGO_MINIMO = (\d+);/, 'CARGO_MINIMO')),
    }),
  },
];

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

const [truth, ...rest] = rows;

console.log('service fee, as each package states it:\n');
for (const r of rows) {
  console.log(`  ${String(r.ratePercent).padStart(3)}%  floor ${pesos(r.floorCentavos).padEnd(10)}  ${r.file}`);
  console.log(`        ${r.label}`);
}
console.log();

for (const r of rest) {
  for (const [field, label] of [['ratePercent', 'rate'], ['floorCentavos', 'floor']]) {
    if (r[field] !== truth[field]) {
      const shown = field === 'floorCentavos' ? [pesos(truth[field]), pesos(r[field])] : [`${truth[field]}%`, `${r[field]}%`];
      console.error(
        `::error file=${r.file}::service fee ${label} is ${shown[1]} here but ${shown[0]} in ` +
          `${truth.file}, which is what a client is actually charged. Whichever is wrong, they cannot ship apart.`,
      );
      failures += 1;
    }
  }
}

if (failures > 0) {
  console.error(`\n${failures} mismatch(es). The fee is one fact; it has ${SITES.length} copies and they must agree.`);
  process.exit(1);
}

console.log(`all ${SITES.length} agree: ${truth.ratePercent}% with a ${pesos(truth.floorCentavos)} floor.`);
