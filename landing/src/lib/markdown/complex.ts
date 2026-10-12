/**
 * The Markdown twin of a complex page (/c/<slug>): the facts a visitor or an agent needs to
 * decide whether to book there, and how. It reads the ComplexFacts the HTML page's data
 * produces, so the two cannot disagree.
 */
import { joinSpanish } from '../copy.ts';
import type { GoneView } from '../complex-gone.ts';
import type { ComplexFacts } from './complex-facts.ts';
import { complexLine } from './hub.ts';
import { absolute, bullets, document, escapeInline, faqBlock, frontMatter, heading, link, SITE } from './format.ts';

const plural = (count: number) => `${count} ${count === 1 ? 'cancha' : 'canchas'}`;

export function complexMarkdown(facts: ComplexFacts, origin: string = SITE): string {
  const booking = `${origin}/c/${facts.slug}`;
  const front = frontMatter({ title: facts.title, description: facts.description, canonical: facts.canonical });
  const where = [facts.address, facts.city, facts.province].filter((part) => part !== '').join(', ');
  const wa = facts.whatsapp ? `https://wa.me/${facts.whatsapp}` : null;

  const faq: Array<[string, string]> = [];
  if (where) faq.push([`¿Dónde queda ${facts.name}?`, `${facts.name} está en ${where}.`]);
  if (facts.sports.length > 0) {
    const sports = joinSpanish(facts.sports.map((sport) => sport.label.toLowerCase()));
    faq.push([`¿Qué deportes se juegan en ${facts.name}?`, `En ${facts.name} hay canchas de ${sports}.`]);
  }
  faq.push([
    `¿Cómo reservo en ${facts.name}?`,
    facts.online
      ? `Reservá online en ${booking}: elegí fecha, deporte y horario.`
      : `Reservá por WhatsApp${wa ? `: ${wa}` : '.'}`,
  ]);
  if (facts.amenities.length > 0) {
    faq.push([`¿Qué servicios tiene ${facts.name}?`, `${joinSpanish(facts.amenities)}.`]);
  }

  return document(front, [
    heading(1, facts.name),
    escapeInline(facts.intro),
    where ? `Dirección: ${escapeInline(where)}.` : null,
    facts.phone ? `Teléfono: ${escapeInline(facts.phone)}${wa ? ` · ${link('WhatsApp', wa)}` : ''}` : null,
    heading(2, 'Canchas'),
    facts.sports.length > 0
      ? bullets(facts.sports.map((sport) => `${escapeInline(sport.label)}: ${plural(sport.courts)}`))
      : 'Este complejo todavía no tiene canchas activas publicadas.',
    heading(2, 'Horarios'),
    facts.hours.length > 0 ? bullets(facts.hours.map((hour) => `${escapeInline(hour.day)}: ${hour.text}`)) : null,
    facts.prices.length > 0 ? heading(2, 'Tarifas de referencia') : null,
    facts.prices.length > 0
      ? 'El precio final de cada turno se confirma al elegir el horario en la reserva.'
      : null,
    facts.prices.length > 0 ? bullets(facts.prices.map(escapeInline)) : null,
    facts.amenities.length > 0 ? heading(2, 'Servicios') : null,
    facts.amenities.length > 0 ? bullets(facts.amenities.map(escapeInline)) : null,
    faq.length > 0 ? heading(2, 'Preguntas frecuentes') : null,
    ...faq.map(([question, answer]) => faqBlock(question, answer)),
    heading(2, 'Enlaces'),
    bullets([link(`Reservar en ${facts.name}`, booking)]),
  ]);
}

/**
 * The Markdown twin of a deleted complex's page (/c/<slug> answering 410): the same headline,
 * lead and other courts as the HTML page, with links to the city and the city index.
 */
export function goneMarkdown(view: GoneView, canonical: string): string {
  const front = frontMatter({ title: view.title, description: view.description, canonical });
  const hasAlternatives = view.alternatives.length > 0;
  return document(front, [
    heading(1, view.h1),
    escapeInline(view.lead),
    hasAlternatives ? heading(2, `Otras canchas en ${view.city}`) : null,
    hasAlternatives ? view.alternatives.map(complexLine).join('\n') : null,
    heading(2, 'Más en Vibe'),
    bullets([
      ...(view.cityHref ? [link(`Ver canchas en ${view.city}`, absolute(view.cityHref))] : []),
      link('Ver todas las ciudades', absolute('/canchas')),
    ]),
  ]);
}
