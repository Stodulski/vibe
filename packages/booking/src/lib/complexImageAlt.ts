/** Alt text stays within this many characters: screen readers and search snippets read it whole. */
const ALT_MAX = 125;
/** The sports named in the alt text, most courts first as the page lists them. */
const MAX_ALT_SPORTS = 3;

/** "a", "a y b", "a, b y c". */
function joinSpanish(items: readonly string[]): string {
  if (items.length <= 1) return items.join('');
  return `${items.slice(0, -1).join(', ')} y ${items.at(-1) ?? ''}`;
}

/**
 * The alt text of a complex's cover photo: "Club X, canchas de pádel y fútbol en Rosario".
 * The sports and the city are left out one at a time until the text fits ALT_MAX; the
 * club's name alone is the last resort.
 */
export function complexCoverAlt(input: { name: string; city: string; sports: readonly string[] }): string {
  const name = input.name.trim();
  const city = input.city.trim();
  const place = city === '' ? '' : ` en ${city}`;
  const sports = [
    ...new Set(input.sports.map((sport) => sport.trim().toLowerCase()).filter((sport) => sport !== '')),
  ].slice(0, MAX_ALT_SPORTS);

  const [topSport = ''] = sports;
  const candidates = [
    sports.length > 0 ? `${name}, canchas de ${joinSpanish(sports)}${place}` : '',
    sports.length > 1 ? `${name}, canchas de ${topSport}${place}` : '',
    `${name}${place}`,
  ];
  return candidates.find((alt) => alt !== '' && alt.length <= ALT_MAX) ?? name;
}

/** The alt text of a complex's logo: "Logo de Club X". */
export function complexLogoAlt(name: string): string {
  return `Logo de ${name}`;
}
