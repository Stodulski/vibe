/** Names are composed from characteristics: no screen shows a stored name (S10 to S12, S15). */
export const tournamentsTraits = {
  /** Padel, tennis and pickleball: "Caballeros", "Damas", "Mixto". */
  modality: { men: 'Caballeros', women: 'Damas', mixed: 'Mixto' },
  /** Football, basketball, volleyball and hockey: "Masculino", "Femenino", "Mixto". */
  modalityMasculine: { men: 'Masculino', women: 'Femenino', mixed: 'Mixto' },
  player: { men: 'Caballero', women: 'Dama' },
  playerMasculine: { men: 'Masculino', women: 'Femenino' },
  levels: ['1ra', '2da', '3ra', '4ta', '5ta', '6ta', '7ma', '8va', '9na'],
  suma: 'Suma',
  tournamentTitle: (sport: string) => `Torneo de ${sport}`,
  weekdays: ['Dom', 'Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb'],
  months: ['ene', 'feb', 'mar', 'abr', 'may', 'jun', 'jul', 'ago', 'sep', 'oct', 'nov', 'dic'],
} as const;
