export const events = {
  title: 'Canchas libres',
  create: 'Crear evento',
  createDescription: 'Bloquea las canchas en ese horario. La gente se anota de a una y paga online.',
  upcoming: 'Próximos',
  past: 'Pasados',
  listLabel: 'Eventos',
  noUpcoming: 'No hay eventos próximos',
  noUpcomingDescription:
    'Creá un evento de canchas libres: las canchas quedan reservadas y la gente se anota y paga online.',
  noPast: 'No hay eventos pasados',
  loadError: 'No pudimos cargar los eventos',
  spotsTaken: (taken: number, capacity: number) => `${String(taken)} de ${String(capacity)} anotados`,
  courtsCount: (count: number) => (count === 1 ? '1 cancha' : `${String(count)} canchas`),
  cancelledBadge: 'Cancelado',
  fullBadge: 'Completo',
  pricePerPerson: (price: string) => `${price} por persona`,
  endsNextDay: 'Termina al día siguiente',
  // The grid bar: "Canchas libres · {día} {hora}".
  blockLabel: (day: string, time: string) => `Canchas libres · ${day} ${time}`,
  blockLabelCompact: 'Canchas libres',

  // Create form
  form: {
    name: 'Nombre del evento',
    namePlaceholder: 'Ej: Americano de los viernes',
    sport: 'Deporte',
    date: 'Fecha',
    startTime: 'Desde',
    endTime: 'Hasta',
    courts: 'Canchas',
    allCourts: 'Todas las canchas',
    courtsHint: 'Elegí en qué canchas se juega. Quedan sin reservas durante el evento.',
    capacity: 'Cupo',
    capacityHint: 'Cuántas personas se pueden anotar.',
    price: 'Precio por persona',
    priceHint: 'Cada persona paga este monto y el costo del servicio de pago se suma aparte.',
    submit: 'Crear evento',
    createSuccess: 'Evento creado',
    createError: 'No pudimos crear el evento',
    selectDate: 'Elegí la fecha',
  },
  validation: {
    nameRequired: 'Ingresá el nombre del evento',
    nameMax: 'El nombre puede tener hasta 100 caracteres',
    courtsRequired: 'Elegí al menos una cancha',
    capacityRange: 'El cupo va de 1 a 500 personas',
    priceRequired: 'Ingresá el precio por persona',
    pricePositive: 'El precio tiene que ser mayor a 0',
    duration: 'El evento dura entre 30 minutos y 12 horas',
  },
  // A 409 on create names what holds the hours and on which court.
  conflict: {
    title: 'Ese horario no está libre',
    booking: (court: string) => `Ya hay una reserva en ese horario en ${court}.`,
    blocked: (court: string) => `Ese horario está bloqueado en ${court}.`,
    fixed: (court: string) => `Un turno fijo ocupa ese horario en ${court}.`,
    event: (court: string) => `Ya hay otro evento en ese horario en ${court}.`,
    unknown: 'Alguna de las canchas elegidas ya está ocupada en ese horario.',
    hint: 'Cambiá el horario o sacá esa cancha del evento.',
  },

  // Detail
  detail: {
    courts: 'Canchas',
    when: 'Cuándo',
    capacity: 'Cupo',
    price: 'Precio por persona',
    participants: 'Participantes',
    participantsCount: (count: number) => (count === 1 ? '1 persona' : `${String(count)} personas`),
    noParticipants: 'Todavía no se anotó nadie',
    loadError: 'No pudimos cargar el evento',
    cancelledNote: 'Este evento está cancelado. Las canchas ya están libres.',
    cancelledAt: (date: string) => `Cancelado el ${date}`,
  },
  participant: {
    held: 'Pagando',
    confirmed: 'Pagó',
    cancelledUnpaid: 'Sin pago',
    refunded: 'Reembolsado',
    refundPending: 'Reembolso en curso',
    partialRefund: 'Reembolso parcial',
    cancelledByOwner: 'Cancelado por vos',
    cancelledByParticipant: 'Canceló la persona',
    paid: (amount: string) => `Pagó ${amount}`,
    refundedAmount: (amount: string) => `Devuelto ${amount}`,
    call: 'Llamar',
  },

  // Cancel
  cancel: {
    action: 'Cancelar evento',
    confirmTitle: 'Cancelar el evento',
    confirmDescription:
      'Las canchas quedan libres y se cancelan todas las inscripciones. A cada persona que ya pagó le devolvemos el pago completo por Mercado Pago, servicio incluido, y le avisamos por email si dejó uno.',
    confirmDescriptionPaid: (paid: number) =>
      `Las canchas quedan libres y se cancelan todas las inscripciones. ${
        paid === 1 ? 'A la persona que ya pagó' : `A las ${String(paid)} personas que ya pagaron`
      } les devolvemos el pago completo por Mercado Pago, servicio incluido, y les avisamos por email si dejaron uno.`,
    confirmLabel: 'Cancelar evento',
    back: 'Volver',
    success: (refunded: number) =>
      refunded === 0
        ? 'Evento cancelado'
        : `Evento cancelado · ${String(refunded)} ${refunded === 1 ? 'reembolso en curso' : 'reembolsos en curso'}`,
    error: 'No pudimos cancelar el evento',
  },
} as const;
