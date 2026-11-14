export const tournaments = {
  title: 'Torneos',
  // The grid bar: "Torneo · {torneo} · {categoría}".
  blockLabel: (name: string) => `Torneo · ${name}`,
  blockLabelCompact: 'Torneo',
} as const;
