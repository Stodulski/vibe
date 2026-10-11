/**
 * Head copy of the complex page: the backend prerender's title and description, the
 * canonical URL, the share image fallback and the JSON-LD escaping. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { complexCanonical, complexHead, jsonLdScript, joinSpanish } from '../src/lib/complex-meta.ts';

const base = {
  slug: 'club-sur',
  name: 'Club Sur',
  city: 'Banfield',
  online: true,
  sports: ['Pádel', 'Tenis'],
  services: ['Estacionamiento', 'Duchas'],
};

test('title names the club and its city, then the app', () => {
  assert.equal(complexHead(base).title, 'Club Sur en Banfield - Reservá tu cancha | Vibe');
});

test('title drops the city when the complex has none', () => {
  assert.equal(complexHead({ ...base, city: '' }).title, 'Club Sur - Reservá tu cancha | Vibe');
});

test('description lists the sports lowercased, then the place and the channel', () => {
  assert.equal(
    complexHead(base).description,
    'Reservá canchas de pádel y tenis en Club Sur, Banfield online. Estacionamiento, Duchas.',
  );
});

test('description says por WhatsApp when online payments are off', () => {
  assert.match(complexHead({ ...base, online: false }).description, / en Club Sur, Banfield por WhatsApp\. /);
});

test('description without sports or services falls back to the generic copy', () => {
  assert.equal(
    complexHead({ ...base, sports: [], services: [] }).description,
    'Reservá tu cancha en Club Sur, Banfield online. Horarios y ubicación.',
  );
});

test('description names at most three sports and three services', () => {
  const head = complexHead({
    ...base,
    sports: ['Pádel', 'Tenis', 'Fútbol', 'Básquet'],
    services: ['A', 'B', 'C', 'D'],
  });
  assert.equal(head.description, 'Reservá canchas de pádel, tenis y fútbol en Club Sur, Banfield online. A, B, C.');
});

test('canonical is the storefront URL of the complex, with the slug encoded', () => {
  assert.equal(complexCanonical('club sur'), 'https://vibe.com.ar/c/club%20sur');
  assert.equal(complexHead(base).canonical, 'https://vibe.com.ar/c/club-sur');
});

test('share image is the cover, else the logo, else the app default', () => {
  assert.equal(complexHead({ ...base, ownImage: 'https://cdn.example/cover.jpg' }).image, 'https://cdn.example/cover.jpg');
  assert.equal(complexHead({ ...base, ownImage: null }).image, 'https://app.vibe.com.ar/logo.png');
  assert.equal(complexHead({ ...base, ownImage: '' }).image, 'https://app.vibe.com.ar/logo.png');
});

test('locale is es_AR', () => {
  assert.equal(complexHead(base).locale, 'es_AR');
});

test('joinSpanish uses y before the last item', () => {
  assert.equal(joinSpanish([]), '');
  assert.equal(joinSpanish(['a']), 'a');
  assert.equal(joinSpanish(['a', 'b']), 'a y b');
  assert.equal(joinSpanish(['a', 'b', 'c']), 'a, b y c');
});

test('jsonLdScript escapes markup so a venue name cannot close the script element', () => {
  const body = jsonLdScript({ name: 'Club </script><b>&' });
  assert.ok(!body.includes('</script>'));
  assert.ok(!body.includes('<'));
  assert.equal(JSON.parse(body).name, 'Club </script><b>&');
});

test('jsonLdScript escapes the JavaScript line separators', () => {
  const body = jsonLdScript({ name: 'a b c' });
  assert.ok(!body.includes(' '));
  assert.ok(!body.includes(' '));
  assert.equal(JSON.parse(body).name, 'a b c');
});
