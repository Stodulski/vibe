/**
 * The copy of a held slot after things go wrong: the cap on slots held at
 * once, the cooldown on hours just held without paying, and the failure page's
 * way back to a booking whose payment was rejected. Spread into `BOOKING_MESSAGES.publicBooking`.
 */
export const HOLD_MESSAGES = {
  holdCooldown: 'Este horario se reservó hace poco desde esta conexión sin completar el pago. Probá en unos minutos o elegí otro horario.',
  holdCap: 'Ya tenés 3 horarios reservándose desde esta conexión. Terminá o soltá uno para elegir otro.',
  paymentFailedHeldDescription:
    'No se cobró nada. Tu turno sigue guardado unos minutos: podés reintentar el pago o liberarlo y elegir otro horario.',
  paymentFailedRetry: 'Reintentar el pago',
  paymentFailedRetryGone: 'Esta reserva ya no se puede pagar. Elegí un horario de nuevo.',
  paymentFailedActionError: 'No pudimos completar la acción. Probá de nuevo.',
  paymentFailedInProcess: 'Mercado Pago todavía está procesando tu pago. Esperá unos minutos antes de volver a intentar.',
  paymentFailedBackToComplex: 'Volver al complejo',
} as const;
