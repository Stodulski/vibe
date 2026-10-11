/**
 * The detectors the SEO gate and the smoke test share: placeholder text and phones that must
 * never reach the storefront, and JSON-LD dates that disagree with the sitemap. Run with
 * `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
// @ts-expect-error build-time .mjs module, shared by scripts/seo-gate.mjs and tests/smoke.mjs
import { dateDriftProblems, junkWordIn, placeholderPhone, placeholderProblemsInHtml } from '../scripts/seo-checks.mjs';

test('a whole word of placeholder text is caught, whatever its case', () => {
  assert.equal(junkWordIn('Club Asdasd en Rosario'), 'asdasd');
  assert.equal(junkWordIn('QWERTY Padel'), 'qwerty');
  assert.equal(junkWordIn('Prueba de cancha'), 'prueba');
  assert.equal(junkWordIn('Cancha test 1'), 'test');
});

test('placeholder words inside a real word are not caught', () => {
  assert.equal(junkWordIn('Testeo de reservas'), null);
  assert.equal(junkWordIn('Asdfghjk'), null);
  assert.equal(junkWordIn('La Zona F5 en Rosario'), null);
});

test('a phone of one repeated digit run, or the 1111 placeholder, is caught', () => {
  assert.equal(placeholderPhone('+541111111111'), true);
  assert.equal(placeholderPhone('+54 9 11 1111-1111'), true);
  assert.equal(placeholderPhone('+54 9 11 2463-8281'), false);
  assert.equal(placeholderPhone(''), false);
});

test('a page title or heading with a placeholder word is reported once per word', () => {
  const html = '<html><head><title>Test | Vibe</title></head><body><h1>Asdasd en Rosario</h1></body></html>';
  const problems = placeholderProblemsInHtml(html);
  assert.equal(problems.length, 2);
  assert.ok(problems.some((p: string) => p.includes('test')));
  assert.ok(problems.some((p: string) => p.includes('asdasd')));
});

test('a clean page reports nothing, and a wa.me link with a placeholder phone is reported', () => {
  assert.deepEqual(placeholderProblemsInHtml('<title>La Zona F5 | Vibe</title><h1>La Zona F5</h1>'), []);
  const html = '<title>X</title><h1>X</h1><a href="https://wa.me/541111111111">WhatsApp</a>';
  assert.equal(placeholderProblemsInHtml(html).length, 1);
});

test('JSON-LD dates that disagree with the sitemap are reported for the pages that carry the page date', () => {
  const problems = dateDriftProblems({
    jsonLd: new Map([['/', '2026-10-10'], ['/privacidad', '2026-10-11'], ['/guias/x', '2026-01-01']]),
    sitemap: new Map([['/', '2026-10-11'], ['/privacidad', '2026-10-11'], ['/guias/x', '2026-10-11']]),
    routes: ['/', '/privacidad'],
  });
  assert.equal(problems.length, 1);
  assert.match(problems[0], /^\/: .*2026-10-10.*2026-10-11/);
});
