/** The copy of the "Avisarme" waitlist. Part of `BOOKING_MESSAGES.waitlist`. */
export const WAITLIST_MESSAGES = {
  notifyMe: 'Avisarme',
  full: 'Completo',
  title: 'Te avisamos cuando se libere',
  openPlayDescription: 'Dejanos tu email y te escribimos apenas se libere un lugar.',
  tournamentDescription: 'Dejanos tu email y te escribimos apenas se libere el lugar de una pareja.',
  /** Followed by the hour and a period: "Te escribimos apenas se libere una cancha a las 19:00." */
  slotAnyCourtPrefix: 'Te escribimos apenas se libere una cancha a las',
  emailLabel: 'Email',
  emailInvalid: 'Revisá el email',
  send: 'Enviar',
  sendSlot: 'Avisarme cuando se libere',
  sending: 'Enviando...',
  close: 'Cerrar',
  cancel: 'Cancelar',
  done: 'Listo, te avisamos por email apenas se libere. El primero que paga se queda con el lugar.',
  error: 'No pudimos guardar tu email. Probá de nuevo.',
  placeFree: 'Ya hay lugar libre: podés inscribirte o reservar ahora.',
  slotGone: 'Ese horario ya no se puede esperar. Elegí otro.',
  capReached: 'Ya estás esperando muchos lugares en este complejo.',
  tooMany: 'Probá de nuevo en un rato.',
  takenFirst: 'Alguien reservó ese turno antes que vos. Si se vuelve a liberar, te avisamos de nuevo por email.',
} as const;
