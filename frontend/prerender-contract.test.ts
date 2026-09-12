// @vitest-environment node
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';

/**
 * The backend prerenders a complex's public page by fetching this index.html
 * and running `strings.ReplaceAll` over it with the literals below. A
 * substitution that finds nothing is silent: the crawler still gets a 200 with
 * the generic Vibe copy, and nobody notices until a share preview is wrong.
 *
 * That is exactly what happened — the frontend had accented copy and an
 * absolute og:image while the Go constants had neither, so three of the five
 * placeholders never matched and only the <title> was ever personalised.
 *
 * These literals are a hand-copy of the `placeholder*` constants in
 * backend/internal/publicsite/copy.go. They live here so a future edit to
 * index.html breaks the frontend's own CI rather than the prerender in
 * production. Changing either side means changing both.
 */
const BACKEND_PLACEHOLDERS = {
  titleTag: '<title>Vibe</title>',
  description: 'content="Vibe - Gestion de complejos deportivos, reservas y canchas"',
  ogTitle: 'content="Vibe - Reserva tu cancha"',
  ogDescription: 'content="Reserva canchas de padel, tenis y futbol de forma rapida y segura."',
  image: 'content="/logo.png"',
};

const indexHtml = readFileSync(fileURLToPath(new URL('./index.html', import.meta.url)), 'utf8');

describe('prerender placeholders', () => {
  it.each(Object.entries(BACKEND_PLACEHOLDERS))('index.html still contains the %s placeholder', (_name, literal) => {
    expect(indexHtml).toContain(literal);
  });

  it('ships no canonical of its own, so the prerender leaves exactly one', () => {
    // The prerender injects `<link rel="canonical">` for the complex's own URL
    // before </head> without touching what is already there. A canonical
    // hardcoded in the shell survived alongside it, so every prerendered page
    // went out with two — one of them pointing at the landing, which is not
    // this page. The shell ships none; `useCanonical` creates the right one in
    // the SPA, on public and private routes alike.
    expect(indexHtml).not.toContain('rel="canonical"');
  });
});
