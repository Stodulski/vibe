/**
 * The Markdown twin of a city hub (/canchas/<city>). It reads the HubView the HTML page
 * renders, so the complexes, the FAQ and the links are the same ones.
 */
import type { HubView, HubComplexView } from '../hub.ts';
import { absolute, bullets, document, escapeInline, faqBlock, frontMatter, heading, link, SITE } from './format.ts';

function complexLine(complex: HubComplexView): string {
  const details = [complex.address, complex.courtsLine ?? (complex.sports.length ? complex.sports.join(', ') : '')]
    .filter((part) => part !== '')
    .map(escapeInline);
  const tail = details.length ? `: ${details.join(' · ')}` : '';
  return `- ${link(complex.name, absolute(complex.href))}${tail}`;
}

export function hubMarkdown(view: HubView, origin: string = SITE): string {
  const front = frontMatter({ title: view.title, description: view.description, canonical: view.canonical });
  const where = [view.city, view.province].filter((part) => part !== '').map(escapeInline).join(', ');
  const place = `Ubicación: ${where}.`;

  return document(front, [
    heading(1, view.h1),
    place,
    escapeInline(view.intro),
    heading(2, `Complejos en ${view.city}`),
    view.complexes.length === 0
      ? 'Todavía no hay complejos de Vibe en esta ciudad.'
      : view.complexes.map(complexLine).join('\n'),
    view.faq.length > 0 ? heading(2, 'Preguntas frecuentes') : null,
    ...view.faq.map((item) => faqBlock(item.question, item.answer)),
    view.otherHubs.length > 0 ? heading(2, 'Canchas en otras ciudades') : null,
    view.otherHubs.length > 0
      ? bullets(view.otherHubs.map((other) => link(other.label, absolute(other.href, origin))))
      : null,
    heading(2, 'Más en Vibe'),
    bullets([link('Todas las ciudades', `${origin}/canchas`), link('Inicio', `${origin}/`)]),
  ]);
}
