/**
 * The Markdown twin of the city index (/canchas). It reads the HubIndexView the HTML page
 * renders, grouped by province in the same order.
 */
import type { HubIndexView } from '../hub-index.ts';
import { absolute, bullets, document, frontMatter, heading, link, SITE } from './format.ts';

export function hubIndexMarkdown(view: HubIndexView, origin: string = SITE): string {
  const front = frontMatter({ title: view.title, description: view.description, canonical: view.canonical });

  const groups = view.groups.flatMap((group) => [
    heading(2, group.heading),
    bullets(
      group.hubs.map((hub) => {
        const sports = hub.sportsLine ? `${hub.sportsLine} · ` : '';
        return `${link(hub.label, absolute(hub.href, origin))}: ${sports}${hub.complexLine}`;
      }),
    ),
  ]);

  return document(front, [
    heading(1, view.h1),
    view.intro,
    ...groups,
    heading(2, 'Más en Vibe'),
    bullets([link('Inicio', `${origin}/`)]),
  ]);
}
