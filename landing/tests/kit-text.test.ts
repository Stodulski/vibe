/**
 * The words of the kit pictures: the headlines, the steps and the link the pieces show, in the
 * exact Spanish of the kit brief. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { kitText } from '../src/lib/kit/kit-text.ts';

const facts = {
  name: 'La Zona F5',
  city: 'Caleta Olivia',
  province: 'Santa Cruz',
  sports: ['Pádel', 'Fútbol'],
};

test('the reservar piece says reserve online, scan the code or enter the link, and lists the sports', () => {
  const text = kitText(facts, 'la-zona-f5', 'reservar');
  assert.equal(text.name, 'La Zona F5');
  assert.equal(text.headline, 'Reservá tu cancha online');
  assert.equal(text.scanLine, 'Escaneá el código o entrá a');
  assert.equal(text.link, 'vibe.com.ar/c/la-zona-f5');
  assert.equal(text.sports, 'Pádel · Fútbol');
  assert.deepEqual(text.steps, []);
});

test('the como-reservar piece has three steps and the link in the first one', () => {
  const text = kitText(facts, 'la-zona-f5', 'como-reservar');
  assert.equal(text.headline, 'Reservá en 3 pasos');
  assert.deepEqual(text.steps, [
    'Entrá a vibe.com.ar/c/la-zona-f5',
    'Elegí deporte, horario y cancha',
    'Confirmá y te llega el turno por email',
  ]);
  assert.equal(text.scanLine, '');
  assert.equal(text.sports, '');
});

test('the QR URL is the complex page on the storefront origin, and the link shows it without the scheme', () => {
  const text = kitText(facts, 'la-zona-f5', 'reservar');
  assert.equal(text.url, 'https://vibe.com.ar/c/la-zona-f5');
  assert.equal(text.link, text.url.replace('https://', ''));
});

test('the slug is encoded in the URL', () => {
  assert.equal(kitText(facts, 'a b', 'reservar').url, 'https://vibe.com.ar/c/a%20b');
});

test('the name keeps its words and drops the stray spaces around them', () => {
  assert.equal(kitText({ ...facts, name: '  La   Zona  F5 ' }, 'la-zona-f5', 'reservar').name, 'La Zona F5');
});

test('the sports line is empty when the complex has no active sport', () => {
  assert.equal(kitText({ ...facts, sports: [] }, 'la-zona-f5', 'reservar').sports, '');
});

test('the copy never carries an exclamation mark, an inverted one or an em dash', () => {
  for (const piece of ['reservar', 'como-reservar'] as const) {
    const text = kitText(facts, 'la-zona-f5', piece);
    const copy = [text.name, text.headline, text.scanLine, text.link, text.sports, ...text.steps].join('\n');
    assert.doesNotMatch(copy, /[!¡—]/, piece);
  }
});
