/**
 * The copy of what a player declares at sign-up (gender and level) and of the
 * rules it is checked against. Part of `BOOKING_MESSAGES.declaration`. The
 * functions take the words of a level or a range, already named by
 * `lib/traitLabels`.
 */
export const DECLARATION_MESSAGES = {
  genderLabel: 'Caballero / Dama',
  genderMen: 'Caballero',
  genderWomen: 'Dama',
  genderRequired: 'Elegí si es caballero o dama.',
  levelLabel: 'Categoría',
  levelPlaceholder: 'Elegí',
  levelRequired: 'Elegí su categoría.',
  pairMenOnly: 'Esta categoría es de Caballeros: los dos jugadores tienen que ser caballeros.',
  pairWomenOnly: 'Esta categoría es de Damas: los dos jugadores tienen que ser damas.',
  pairMixed: 'Es mixta: tiene que ser un caballero y una dama.',
  levelTooStrong: (level: string) =>
    `Esta categoría es ${level}: no pueden anotarse jugadores de una categoría más alta.`,
  levelOrWeaker: (level: string) => `${level} o menor`,
  sumaTooLow: (suma: number) => `La suma de las dos categorías tiene que ser ${String(suma)} o más.`,
  sumaRequirement: (suma: number) =>
    `Suma ${String(suma)}: la suma de las dos categorías tiene que ser ${String(suma)} o más.`,
  openPlayMenOnly: 'Esta cancha abierta es solo de Caballeros.',
  openPlayWomenOnly: 'Esta cancha abierta es solo de Damas.',
  openPlayRange: (range: string) => `Esta cancha abierta es de ${range}.`,
  openPlaySolo: (level: string) => `Esta cancha abierta es solo de ${level}.`,
} as const;
