/**
 * The llmstxt.org index for the site: what Vibe is, and where an agent should read first.
 *
 * /llms.txt (src/pages/llms.txt.ts) renders it on every request, so a city or complex created
 * since the last deploy is listed without one. The static part is src/data/llms-static.json,
 * which the build writes from the built pages. Each entry is `- [title](url): note — Markdown:
 * <twin>`, and a section is written only when it has entries, so a missing API leaves out the
 * dynamic sections and nothing else.
 */
import type { PublicApi } from '../api.ts';
import { cityPath, complexPath } from '../hub.ts';
import type { HubIndex } from '../hub-index.ts';
import { STOREFRONT_ORIGIN } from '../storefront.ts';

export interface LlmsEntry {
  title: string;
  url: string;
  /** The URL of the entry's Markdown twin. */
  md: string;
  note: string;
}

/** The part of the index that comes from the built pages, not from the API. */
export interface LlmsStatic {
  /** H1, "Vibe". */
  title: string;
  /** The home description, the blockquote. */
  description: string;
  /** One paragraph on what the site is. */
  summary: string;
  /** ISO date of the newest content change. */
  updated: string;
  pages: LlmsEntry[];
}

export interface LlmsTxtData extends LlmsStatic {
  cities: LlmsEntry[];
  complexes: LlmsEntry[];
}

/** Receives what went wrong without failing the index. */
export type LlmsReport = (message: string, detail?: unknown) => void;

/** A link text must not close its own brackets early. */
const label = (text: string): string => text.replace(/\[/g, '(').replace(/\]/g, ')');

function entry({ title, url, md, note }: LlmsEntry): string {
  const tail = note ? `: ${note}` : '';
  const twin = md ? ` — Markdown: ${md}` : '';
  return `- [${label(title)}](${url})${tail}${twin}`;
}

function section(heading: string, entries: LlmsEntry[]): string | null {
  if (!entries.length) return null;
  return `## ${heading}\n\n${entries.map(entry).join('\n')}`;
}

/** Writes the index. Pure: the caller gathers the entries. */
export function renderLlmsTxt({ title, description, summary, updated, pages, cities, complexes }: LlmsTxtData): string {
  const intro = [
    `# ${title}`,
    '',
    `> ${description}`,
    '',
    summary,
    '',
    'Toda página de este sitio responde en Markdown si la pedís con la cabecera ' +
      '`Accept: text/markdown`, y en HTML si no. Los archivos `.md` listados abajo son ' +
      'ese mismo contenido, accesible directamente.',
    '',
    `Última actualización: ${updated}`,
  ];

  const sections = [
    section('Páginas', pages),
    section('Canchas', cities),
    section('Complejos', complexes),
  ].filter((text): text is string => text !== null);

  return `${intro.join('\n')}\n\n${sections.join('\n\n')}\n`;
}

/**
 * The city and complex sections, read through the public API. The index names every city, and a
 * city whose details cannot be read keeps its entry and loses only its complexes. When the city
 * list itself cannot be read, both sections are left out. Failures are reported, never thrown.
 */
export async function dynamicLlmsEntries(
  api: PublicApi,
  report: LlmsReport,
): Promise<{ cities: LlmsEntry[]; complexes: LlmsEntry[] }> {
  let index: HubIndex;
  try {
    index = await api.hubIndex();
  } catch (error) {
    report('llms.txt  API no disponible, se omiten ciudades y complejos', error);
    return { cities: [], complexes: [] };
  }

  const details = await Promise.allSettled(index.hubs.map((hub) => api.hubData(hub.slug)));
  const cities: LlmsEntry[] = [{
    title: 'Canchas en Vibe',
    url: `${STOREFRONT_ORIGIN}/canchas`,
    md: `${STOREFRONT_ORIGIN}/canchas.md`,
    note: 'Todas las ciudades con complejos',
  }];
  const complexes: LlmsEntry[] = [];

  index.hubs.forEach((hub, i) => {
    const province = hub.province ? `${hub.province} · ` : '';
    cities.push({
      title: `Canchas en ${hub.name}`,
      url: `${STOREFRONT_ORIGIN}${cityPath(hub.slug)}`,
      md: `${STOREFRONT_ORIGIN}/canchas-${hub.slug}.md`,
      note: `${province}${hub.complexCount} complejo(s)`,
    });

    const detail = details[i];
    if (detail.status !== 'fulfilled') {
      report(`llms.txt  no se pudo leer ${hub.slug}`, detail.reason);
      return;
    }
    for (const complex of detail.value.complexes) {
      complexes.push({
        title: complex.name,
        url: `${STOREFRONT_ORIGIN}${complexPath(complex.slug)}`,
        md: `${STOREFRONT_ORIGIN}/c-${complex.slug}.md`,
        note: [complex.address, hub.name].filter(Boolean).join(', '),
      });
    }
  });

  return { cities, complexes };
}

/** The whole index for one request: the static part from the build, the dynamic sections from the API. */
export async function llmsTxt(base: LlmsStatic, api: PublicApi, report: LlmsReport): Promise<string> {
  const { cities, complexes } = await dynamicLlmsEntries(api, report);
  return renderLlmsTxt({ ...base, cities, complexes });
}
