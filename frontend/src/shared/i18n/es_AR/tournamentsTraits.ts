/** Names are composed from characteristics: no screen shows a stored name (S10 to S12). */
export const tournamentsTraits = {
  modality: { men: 'Caballeros', women: 'Damas', mixed: 'Mixto' },
  player: { men: 'Caballero', women: 'Dama' },
  levels: ['1ra', '2da', '3ra', '4ta', '5ta', '6ta', '7ma', '8va', '9na'],
  suma: 'Suma',
  tournamentTitle: (sport: string) => `Torneo de ${sport}`,
  weekdays: ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb'],
  months: ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic'],
} as const;
