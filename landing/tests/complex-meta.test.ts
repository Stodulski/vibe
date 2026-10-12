/**
 * Head copy of the complex page: the backend prerender's title and description, the
 * canonical URL, the share image fallback and the JSON-LD escaping. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
  complexCanonical,
  complexHead,
  goneDescription,
  goneHeadline,
  goneLead,
  goneTitle,
  jsonLdScript,
  joinSpanish,
} from '../src/lib/complex-meta.ts';
import { DEFAULT_SHARE_IMAGE } from '../src/lib/storefront.ts';

const base = {
  slug: 'club-sur',
  name: 'Club Sur',
  city: 'Banfield',
  online: true,
  sports: ['Pádel', 'Tenis'],
  services: ['Estacionamiento', 'Duchas'],
};

test('title names the complex, its sports and its city, then the app', () => {
  assert.equal(complexHead(base).title, 'Club Sur: canchas de pádel y tenis en Banfield | Vibe');
});

test('title falls back to the top sport when the full title is over 60 characters', () => {
  assert.equal(
    complexHead({ ...base, name: 'Club Sur Norte', sports: ['Pádel', 'Tenis', 'Fútbol'] }).title,
    'Club Sur Norte: canchas de pádel en Banfield | Vibe',
  );
});

test('title keeps the generic copy when even the top sport does not fit, or there are no sports', () => {
  assert.equal(
    complexHead({
      ...base,
      name: 'Complejo Muy Largo Con Nombre Extenso Para Probar',
      city: 'Ciudad Con Nombre Muy Largo Tambien',
      sports: ['Pádel', 'Tenis'],
    }).title,
    'Complejo Muy Largo Con Nombre Extenso Para Probar en Ciudad Con Nombre Muy Largo Tambien - Reservá tu cancha | Vibe',
  );
  assert.equal(complexHead({ ...base, sports: [] }).title, 'Club Sur en Banfield - Reservá tu cancha | Vibe');
});

test('description counts the courts when the payload has them', () => {
  assert.equal(
    complexHead({ ...base, courtCount: 3 }).description,
    'Reservá 3 canchas de pádel y tenis en Club Sur, Banfield online. Estacionamiento, Duchas.',
  );
  assert.equal(
    complexHead({ ...base, courtCount: 1 }).description,
    'Reservá 1 cancha de pádel y tenis en Club Sur, Banfield online. Estacionamiento, Duchas.',
  );
  assert.equal(
    complexHead({ ...base, courtCount: 0 }).description,
    'Reservá canchas de pádel y tenis en Club Sur, Banfield online. Estacionamiento, Duchas.',
  );
});

test('description stays within 155 characters by dropping the services before the place', () => {
  const head = complexHead({
    ...base,
    courtCount: 3,
    services: [
      'Estacionamiento cubierto con acceso para autos grandes',
      'Duchas con agua caliente las veinticuatro horas',
      'Bar y buffet con comida casera',
    ],
  });
  assert.equal(head.description, 'Reservá 3 canchas de pádel y tenis en Club Sur, Banfield online.');
  assert.ok(head.description.length <= 155);
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

test('share image is the cover, else the logo, else the site share image', () => {
  assert.equal(complexHead({ ...base, ownImage: 'https://cdn.example/cover.jpg' }).image, 'https://cdn.example/cover.jpg');
  assert.equal(complexHead({ ...base, ownImage: null }).image, 'https://vibe.com.ar/og-image.png');
  assert.equal(complexHead({ ...base, ownImage: '' }).image, 'https://vibe.com.ar/og-image.png');
});

test('the fallback share image is the 1200x630 site image, not the 300x300 app logo', () => {
  assert.equal(DEFAULT_SHARE_IMAGE, 'https://vibe.com.ar/og-image.png');
});

test('names are trimmed and their inner spaces collapsed before they reach the copy', () => {
  const head = complexHead({ ...base, name: 'La  Zona ', city: ' Rosario ' });
  assert.equal(head.title, 'La Zona: canchas de pádel y tenis en Rosario | Vibe');
  assert.equal(head.description, 'Reservá canchas de pádel y tenis en La Zona, Rosario online. Estacionamiento, Duchas.');
  assert.equal(
    complexHead({ ...base, name: 'Afa ', city: 'Rosario', sports: [] }).title,
    'Afa en Rosario - Reservá tu cancha | Vibe',
  );
});

test('locale is es_AR', () => {
  assert.equal(complexHead(base).locale, 'es_AR');
});

test('the gone title names the club and keeps "por Vibe" while it fits in 60 characters', () => {
  assert.equal(goneTitle('Club Viejo'), 'Club Viejo ya no toma reservas por Vibe');
});

test('the gone title drops "por Vibe" when the full title is over 60 characters', () => {
  const title = goneTitle('Club Deportivo Los Amigos de Rosario');
  assert.equal(title, 'Club Deportivo Los Amigos de Rosario ya no toma reservas');
  assert.ok(title.length <= 60);
});

test('the gone H1 always keeps "por Vibe", even when the title drops it', () => {
  assert.equal(goneHeadline('Club Deportivo Los Amigos de Rosario'), 'Club Deportivo Los Amigos de Rosario ya no toma reservas por Vibe');
});

test('the gone description points to the other courts of the city, or to other cities when there are none', () => {
  assert.equal(goneDescription('Rosario', true), 'Mirá otras canchas en Rosario que podés reservar online en Vibe.');
  assert.equal(goneDescription('Rosario', false), 'Mirá las canchas de otras ciudades en Vibe.');
});

test('the gone lead names the city when it has courts to book, and the other cities when it has none', () => {
  assert.equal(goneLead('Rosario', true), 'Pero hay otras canchas en Rosario donde podés reservar online.');
  assert.equal(goneLead('Rosario', false), 'Mirá las canchas de otras ciudades.');
});

test('the gone copy has no exclamation marks and no dashes', () => {
  const copy = [
    goneHeadline('Club Viejo'),
    goneTitle('Club Viejo'),
    goneDescription('Rosario', true),
    goneDescription('Rosario', false),
    goneLead('Rosario', true),
    goneLead('Rosario', false),
  ];
  for (const text of copy) assert.equal(/[!¡—–]/.test(text), false, text);
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
