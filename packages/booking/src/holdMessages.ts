/**
 * The copy of a held slot after things go wrong: the cooldown on hours just
 * held without paying, and the failure page's way back to a booking whose
 * payment was rejected. Spread into `BOOKING_MESSAGES.publicBooking`.
 */
export const HOLD_MESSAGES = {
  holdCooldown: 'Reservaste este horario hace poco y no se completó el pago. Probá en unos minutos o elegí otro horario.',
  paymentFailedHeldDescription:
    'No se cobró nada. Tu turno sigue guardado unos minutos: podés reintentar el pago o liberarlo y elegir otro horario.',
  paymentFailedRetry: 'Reintentar el pago',
  paymentFailedRetryGone: 'Esta reserva ya no se puede pagar. Elegí un horario de nuevo.',
  paymentFailedActionError: 'No pudimos completar la acción. Probá de nuevo.',
} as const;
