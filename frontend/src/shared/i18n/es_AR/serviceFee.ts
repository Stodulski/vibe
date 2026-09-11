export const serviceFee = {
  label: 'Cargo de servicio',
  // Helper line under the "Cargo de servicio" row in PriceBreakdown —
  // replaces the old micro-text security disclaimer under the submit
  // button, which said the same thing further from the row it was about.
  notDeductedFromCourtPrice: 'No se descuenta del precio de la cancha.',
  ownerInfo:
    'Tus clientes pagan un cargo de servicio (7%, mín. $1.000). Vos solo absorbés el costo de procesamiento de MercadoPago.',
  zeroCost: 'Sin cargos adicionales de la plataforma',
} as const;
