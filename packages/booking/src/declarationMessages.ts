/**
 * The copy of what a player declares at sign-up (gender and level) and of the
 * rules it is checked against. Part of `BOOKING_MESSAGES.declaration`. The
 * functions take the words already named by `lib/traitLabels`: the modality,
 * the gender and the level or range, in the words of the sport. The gender
 * choice's own labels come from `declarationWords`, not from here.
 */
export const DECLARATION_MESSAGES = {
  levelLabel: 'Categoría',
  levelPlaceholder: 'Elegí',
  levelRequired: 'Elegí su categoría.',
  genderRequired: (genderOr: string) => `Elegí si es ${genderOr}.`,
  pairOnly: (who: string, players: string) =>
    `Esta categoría es de ${who}: los dos jugadores tienen que ser ${players}.`,
  pairMixed: (pair: string) => `Es mixta: tiene que ser ${pair}.`,
  levelTooStrong: (level: string) =>
    `Esta categoría es ${level}: no pueden anotarse jugadores de una categoría más alta.`,
  levelOrWeaker: (level: string) => `${level} o menor`,
  sumaTooLow: (suma: number) => `La suma de las dos categorías tiene que ser ${String(suma)} o más.`,
  sumaRequirement: (suma: number) =>
    `Suma ${String(suma)}: la suma de las dos categorías tiene que ser ${String(suma)} o más.`,
  openPlayOnly: (who: string) => `Esta cancha abierta es solo de ${who}.`,
  openPlayRange: (range: string) => `Esta cancha abierta es de ${range}.`,
  openPlaySolo: (level: string) => `Esta cancha abierta es solo de ${level}.`,
} as const;
