/**
 * The kit's QR code: the module matrix of a complex URL and the SVG picture built from it. It
 * checks the shape of the code, not that a scanner reads it. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { qrMatrix, qrSvgDataUrl } from '../src/lib/kit/qr.ts';

const URL_A = 'https://vibe.com.ar/c/la-zona-f5';
const URL_B = 'https://vibe.com.ar/c/otro-complejo';

function svgOf(dataUrl: string): string {
  return Buffer.from(dataUrl.slice(dataUrl.indexOf(',') + 1), 'base64').toString('utf8');
}

test('the matrix of a URL is a square with modules', () => {
  const matrix = qrMatrix(URL_A);
  assert.ok(matrix.length > 0);
  for (const row of matrix) assert.equal(row.length, matrix.length);
});

test('the matrix has both dark and light modules', () => {
  const dark = qrMatrix(URL_A).flat().filter(Boolean).length;
  const total = qrMatrix(URL_A).flat().length;
  assert.ok(dark > 0 && dark < total);
});

test('the SVG data URL is a base64 SVG picture', () => {
  assert.match(qrSvgDataUrl(URL_A), /^data:image\/svg\+xml;base64,[A-Za-z0-9+/]+=*$/);
});

test('the SVG data URL changes with the URL and stays the same for the same URL', () => {
  assert.notEqual(qrSvgDataUrl(URL_A), qrSvgDataUrl(URL_B));
  assert.equal(qrSvgDataUrl(URL_A), qrSvgDataUrl(URL_A));
});

test('the SVG leaves a quiet zone of 4 modules around the code', () => {
  const size = qrMatrix(URL_A).length + 8;
  assert.match(svgOf(qrSvgDataUrl(URL_A)), new RegExp(`viewBox="0 0 ${size} ${size}"`));
});
