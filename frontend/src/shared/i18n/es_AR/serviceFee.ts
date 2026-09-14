// Two sentences, two facts: what the client pays, and what the owner pays. The
// onboarding step sets them on their own lines so the owner reads them as two
// answers rather than one paragraph; the compact cards, which have room for a
// sentence and not for a block, use the joined `ownerInfo` below. Composed
// rather than written twice, so the two forms cannot drift apart.
const ownerInfoClients = 'Tus clientes pagan un cargo de servicio del 7% (mínimo $1.000) al reservar.';
const ownerInfoYou = 'A vos MercadoPago solo te descuenta su comisión.';

export const serviceFee = {
  label: 'Cargo de servicio',
  // Helper line under the "Cargo de servicio" row in PriceBreakdown —
  // replaces the old micro-text security disclaimer under the submit
  // button, which said the same thing further from the row it was about.
  notDeductedFromCourtPrice: 'No se descuenta del precio de la cancha.',
  ownerInfoClients,
  ownerInfoYou,
  ownerInfo: `${ownerInfoClients} ${ownerInfoYou}`,
  zeroCost: 'Sin cargos adicionales de la plataforma',
} as const;
