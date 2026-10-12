/**
 * The words the share image draws: the name, the place and the sports line. Run with
 * `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { complexCardText } from '../src/lib/og/complex-card.ts';

const facts = {
  name: 'La Zona F5',
  city: 'Caleta Olivia',
  province: 'Santa Cruz',
  sports: ['Pádel', 'Fútbol'],
};

test('the place is the city and the province, separated by a comma', () => {
  assert.equal(complexCardText(facts).place, 'Caleta Olivia, Santa Cruz');
});

test('the place keeps whichever of city and province the complex has', () => {
  assert.equal(complexCardText({ ...facts, province: '' }).place, 'Caleta Olivia');
  assert.equal(complexCardText({ ...facts, city: '' }).place, 'Santa Cruz');
  assert.equal(complexCardText({ ...facts, city: '', province: '' }).place, '');
});

test('the sports line joins the sports with a middle dot, in the order given', () => {
  assert.equal(complexCardText(facts).sports, 'Pádel · Fútbol');
});

test('the sports line is empty when the complex has no active sport', () => {
  assert.equal(complexCardText({ ...facts, sports: [] }).sports, '');
});

test('the name keeps its words and drops the stray spaces around them', () => {
  assert.equal(complexCardText({ ...facts, name: '  La   Zona  F5 ' }).name, 'La Zona F5');
});

test('the copy the card adds never carries an exclamation mark', () => {
  const card = complexCardText(facts);
  for (const text of [card.place, card.sports]) assert.equal(/[!¡]/.test(text), false, text);
});
