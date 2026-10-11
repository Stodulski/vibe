/**
 * The city index at /canchas: the payload contract of GET /api/v1/public/hubs and the
 * view model the page renders, with its JSON-LD. Pure, like hub.ts.
 */
import { cutAtWord, joinSpanish, jsonLdScript, normalizeText } from './copy.ts';
import {
  breadcrumbJsonLd,
  cityPath,
  countField,
  HUB_DESCRIPTION_MAX,
  InvalidHubData,
  isRecord,
  labelList,
  namesList,
  sportLabel,
  textField,
} from './hub.ts';
import { STOREFRONT_ORIGIN } from './storefront.ts';

const INDEX_TITLE = 'Canchas para reservar por ciudad | Vibe';
const INDEX_H1 = 'Canchas por ciudad';
const NO_PROVINCE_HEADING = 'Otras';

export interface IndexHub {
  slug: string;
  name: string;
  /** Empty when the city has no province on record. */
  province: string;
  complexCount: number;
  /** Spanish labels of the sports the city's complexes offer. */
  sports: string[];
}

export interface HubIndex {
  hubs: IndexHub[];
}

export interface IndexHubView {
  slug: string;
  name: string;
  href: string;
  label: string;
  /** The sports, joined "a y b". Empty when the city has none on record. */
  sportsLine: string;
  /** "1 complejo" or "2 complejos". */
  complexLine: string;
}

export interface IndexGroup {
  heading: string;
  hubs: IndexHubView[];
}

export interface HubIndexView {
  title: string;
  h1: string;
  /** The intro sentence, built from the payload. */
  intro: string;
  description: string;
  canonical: string;
  groups: IndexGroup[];
  /** Serialized JSON-LD: the ItemList of the cities and the BreadcrumbList. */
  jsonLd: string[];
}

function plural(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`;
}

function parseIndexHub(input: unknown): IndexHub {
  if (!isRecord(input) || typeof input.slug !== 'string' || typeof input.name !== 'string') {
    throw new InvalidHubData('a hub needs a slug and a name');
  }
  return {
    slug: input.slug,
    name: normalizeText(input.name),
    province: textField(input.province),
    complexCount: countField(input.complex_count) ?? 0,
    sports: labelList(input.sports).map((sport) => sportLabel(sport)),
  };
}

/** Validates the list of cities. A city without counts or sports still parses. */
export function parseHubIndex(input: unknown): HubIndex {
  if (!isRecord(input) || !Array.isArray(input.hubs)) throw new InvalidHubData('the body needs a list of hubs');
  return { hubs: input.hubs.map(parseIndexHub) };
}

/** Cities by province, in Spanish order; the cities without a province come last, as "Otras". */
function groupByProvince(hubs: readonly IndexHub[]): Array<{ heading: string; hubs: IndexHub[] }> {
  const byProvince = new Map<string, IndexHub[]>();
  for (const hub of hubs) {
    byProvince.set(hub.province, [...(byProvince.get(hub.province) ?? []), hub]);
  }
  const provinces = [...byProvince.keys()].sort((a, b) => {
    if (a === b) return 0;
    if (a === '') return 1;
    if (b === '') return -1;
    return a.localeCompare(b, 'es');
  });
  return provinces.map((province) => ({
    heading: province === '' ? NO_PROVINCE_HEADING : province,
    hubs: [...(byProvince.get(province) ?? [])].sort((a, b) => a.name.localeCompare(b.name, 'es')),
  }));
}

function indexHubView(hub: IndexHub): IndexHubView {
  return {
    slug: hub.slug,
    name: hub.name,
    href: cityPath(hub.slug),
    label: `Canchas en ${hub.name}`,
    sportsLine: joinSpanish(hub.sports),
    complexLine: plural(hub.complexCount, 'complejo', 'complejos'),
  };
}

/** Maps the list of cities to the index page, with its copy and JSON-LD. */
export function toHubIndexView(data: HubIndex, origin: string = STOREFRONT_ORIGIN): HubIndexView {
  const groups = groupByProvince(data.hubs).map((group) => ({
    heading: group.heading,
    hubs: group.hubs.map(indexHubView),
  }));
  const ordered = groups.flatMap((group) => group.hubs);
  const canonical = `${origin}/canchas`;

  if (ordered.length === 0) {
    const intro = 'Todavía no hay ciudades con complejos de Vibe.';
    return {
      title: INDEX_TITLE,
      h1: INDEX_H1,
      intro,
      description: intro,
      canonical,
      groups: [],
      jsonLd: emptyIndexJsonLd(origin, canonical),
    };
  }

  const complexes = data.hubs.reduce((sum, hub) => sum + hub.complexCount, 0);
  const intro = `Elegí tu ciudad y reservá en ${plural(complexes, 'complejo', 'complejos')} de ${plural(
    ordered.length,
    'ciudad',
    'ciudades',
  )}.`;
  const description = cutAtWord(
    `Canchas en ${namesList(ordered.map((hub) => hub.name))}. Elegí tu ciudad y reservá online en Vibe.`,
    HUB_DESCRIPTION_MAX,
  );

  const itemList = jsonLdScript({
    '@context': 'https://schema.org',
    '@type': 'ItemList',
    name: INDEX_H1,
    itemListElement: ordered.map((hub, index) => ({
      '@type': 'ListItem',
      position: index + 1,
      name: hub.label,
      url: `${origin}${hub.href}`,
    })),
  });

  return {
    title: INDEX_TITLE,
    h1: INDEX_H1,
    intro,
    description,
    canonical,
    groups,
    jsonLd: [
      itemList,
      breadcrumbJsonLd([
        { name: 'Inicio', url: `${origin}/` },
        { name: 'Canchas', url: canonical },
      ]),
    ],
  };
}

function emptyIndexJsonLd(origin: string, canonical: string): string[] {
  return [
    jsonLdScript({ '@context': 'https://schema.org', '@type': 'ItemList', name: INDEX_H1, itemListElement: [] }),
    breadcrumbJsonLd([
      { name: 'Inicio', url: `${origin}/` },
      { name: 'Canchas', url: canonical },
    ]),
  ];
}
