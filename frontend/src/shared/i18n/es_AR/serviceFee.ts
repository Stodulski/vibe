export const serviceFee = {
  label: 'Cargo de servicio',
  // Helper line under the "Cargo de servicio" row in PriceBreakdown —
  // replaces the old micro-text security disclaimer under the submit
  // button, which said the same thing further from the row it was about.
  notDeductedFromCourtPrice: 'No se descuenta del precio de la cancha.',
  ownerInfo:
    'Tus clientes pagan un cargo de servicio del 7% (mínimo $1.000) al reservar. A vos MercadoPago solo te descuenta su comisión.',
  zeroCost: 'Sin cargos adicionales de la plataforma',
} as const;
