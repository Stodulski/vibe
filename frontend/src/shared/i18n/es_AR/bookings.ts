export const bookings = {
  clientBlocked: 'Este cliente está bloqueado. Desbloquealo desde Clientes para cargarle una reserva.',
  title: 'Reservas',
  create: 'Nueva reserva',
  edit: 'Editar reserva',
  cancel: 'Cancelar reserva',
  confirmPayment: 'Confirmar pago',
  confirmRemainingPayment: 'Confirmar pago restante',
  remainingBalance: 'Saldo restante',
  remainderPayment: 'Resto',
  paymentOptionLabel: 'Cobro al crear',
  paymentOptions: {
    unpaid: 'Sin cobro',
    deposit: 'Seña',
    full: 'Pago completo',
  },
  depositAmountLabel: 'Monto de la seña',
  depositMax: 'Máx:',
  manualPriceLabel: 'Precio de la reserva',
  manualPriceHint: 'No hay una tarifa configurada para este horario',
  refund: 'Reembolsar',
  refunded: 'Reembolsado',
  serviceFeeSuffix: 'de costo de servicio',
  date: 'Fecha',
  time: 'Horario',
  court: 'Cancha',
  client: 'Cliente',
  status: 'Estado de la reserva',
  paymentStatus: 'Estado del pago',
  price: 'Precio',
  total: 'Total',
  deposit: 'Seña',
  notes: 'Notas',
  duration: 'Duración',
  payment: 'Pago',
  noBookings: 'No hay reservas para esta fecha',
  noBookingsDescription: 'Creá una reserva para comenzar.',
  today: 'Hoy',
  cancelReason: 'Motivo de cancelación',
  cancelReasonPlaceholder: 'Motivo de la cancelación...',
  cancelConfirm: '¿Estás seguro que querés cancelar esta reserva? Esta acción no se puede deshacer.',
  cancelSuccess: 'Reserva cancelada',
  // The server's own refund sentences are written for the client who booked
  // ("el complejo tiene que devolverte…"), which reads as nonsense on the
  // dashboard, where the reader *is* the complex. Same outcomes, told from
  // this side of the counter — keyed off `refund.status`, the server's
  // machine-readable state.
  refundOwner: {
    none: 'No había ningún pago para devolver.',
    not_eligible: 'La cancelación quedó fuera del plazo, así que no corresponde devolver la seña.',
    issued: 'La devolución se envió a Mercado Pago y se acredita en los próximos días hábiles.',
    already_issued: 'La devolución ya se había enviado.',
    queued: 'La devolución está en curso. Te avisamos por mail cuando se complete.',
    manual: 'Tenés que devolverle esta plata al cliente a mano.',
  },
  refundOwnerManualPart: 'La parte en efectivo se la tenés que devolver a mano.',
  // The detail sheet's refund summary: what a cancelled booking still owes
  // the client by hand, and how it gets there.
  refundOwedLabel: 'Monto a devolver',
  refundOwedHint: 'Devolvéselo al cliente. Cuando lo hagas, marcá la devolución como realizada.',
  refundInProgressLabel: 'Devolución',
  refundWaitsForMp:
    'Primero tiene que terminar la devolución de Mercado Pago. Después marcás como realizada la parte que devolvés vos.',
  cancelError: 'Error al cancelar la reserva',
  confirmPaymentError: 'Error al confirmar el pago',
  updateError: 'Error al actualizar la reserva',
  createSuccess: 'Reserva creada exitosamente',
  createError: 'Error al crear la reserva',
  slotOccupied: 'El horario ya está ocupado',
  updateSuccess: 'Reserva actualizada',
  paymentConfirmed: 'Pago confirmado',
  paymentMethod: 'Método de pago',
  paymentAmount: 'Monto',
  clientPhone: 'Teléfono',
  clientFirstName: 'Nombre',
  clientLastName: 'Apellido',
  clientData: 'Datos del cliente',
  selectCourt: 'Seleccioná una cancha',
  selectDateAndCourt: 'Seleccioná fecha y cancha',
  selectDateFirst: 'Seleccioná una fecha',
  confirmCancel: 'Confirmar cancelación',
  bookingTotal: 'Total de la reserva',
  turnDuration: 'Duración del turno',
  closedToday: 'Cerrado este día',
  selectDate: 'Seleccionar fecha',
  unspecified: 'Sin especificar',
  internalNotes: 'Notas internas...',
  statuses: {
    pending: 'Sin confirmar',
    confirmed: 'Confirmada',
    cancelled: 'Cancelada',
    completed: 'Se presentó',
    no_show: 'No se presentó',
    finished: 'Turno finalizado',
  },
  paymentStatuses: {
    unpaid: 'Sin pagos',
    deposit_paid: 'Seña pagada',
    outstanding: 'Pago incompleto',
    fully_paid: 'Pago completo',
    refunded: 'Reembolsado',
    refund_pending: 'Reembolso en curso',
    partial_refund: 'Falta devolver',
  },
  paymentMethods: {
    cash: 'Efectivo',
    transfer: 'Transferencia',
    debit_card: 'Débito',
    credit_card: 'Crédito',
    qr_wallet: 'QR / billetera',
    mercadopago: 'Mercado Pago',
  },
  previousDay: 'Día anterior',
  nextDay: 'Día siguiente',
  // A full phrase, not the calendar arrow's "Día siguiente" above (a
  // different thing: that names a button, this augments a booking's
  // accessible name) — spoken alone as one item in a comma-joined list of
  // facts, "Día siguiente" reads as a command; "Termina al día siguiente"
  // reads as the fact it is. Only ever read aloud: the visible time range no
  // longer carries a "Día sig." marker of its own (`formatHourRange`), so
  // this is the one place a screen reader still learns a booking crosses
  // midnight. See `BookingBlock`'s `bookingAriaLabel`.
  endsNextDay: 'Termina al día siguiente',
  goToToday: 'Ir a hoy',
  chargeType: 'Tipo de cobro',
  fullPayment: 'Pago completo',
  depositPayment: 'Seña',
  depositAmountInput: 'Monto de la seña',
  blockedSlot: 'Horario bloqueado',
  currentTime: 'Hora actual',
  timelineFreeSlot: 'Crear reserva',
  minutesShort: 'min',
  markManualRefund: 'Marcar devolución realizada',
  manualRefundConfirmBefore: 'Confirmás que le devolviste',
  manualRefundConfirmAfter: 'al cliente?',
  manualRefundSuccess: 'Devolución registrada',
  manualRefundError: 'Error al registrar la devolución',
  createBookingStepProgress: 'Progreso de la nueva reserva',
  createBookingStep1: 'Cuándo y dónde',
  createBookingStep2: 'Cliente',
  createBookingStep3: 'Pago',
  // Turnos fijos: a booking that repeats every week on the same weekday and hour.
  fixedBooking: 'Turno fijo',
  openPlayEventSlotTaken: 'Ese horario es parte de un evento de canchas abiertas.',
  tournamentMatchSlotTaken: 'Ese horario lo ocupa un partido de torneo.',
  // The notice above the grid on a day of a tournament that still has matches to schedule.
  unscheduledMatches: {
    message: (tournament: string, count: number) =>
      `${tournament}: ${String(count)} ${count === 1 ? 'partido' : 'partidos'} sin cancha ni horario`,
    schedule: 'Programar',
  },
  series: {
    repeatWeekly: 'Repetir todas las semanas',
    endLabel: 'Hasta cuándo',
    endNever: 'Sin fecha de fin',
    endUntil: 'Hasta',
    endWeeks: 'Por N semanas',
    weeksLabel: 'Cantidad de semanas',
    untilRequired: 'Elegí la fecha de fin',
    untilBeforeStart: 'La fecha de fin no puede ser anterior a la primera fecha',
    weeksRange: 'Ingresá entre 2 y 52 semanas',
    paymentNote: 'Cada turno se cobra como una reserva normal.',
    conflictsTitle: (court: string, count: number, weekdayPlural: string, weekdaySingular: string) =>
      `${court} está ocupada ${String(count)} ${count === 1 ? weekdaySingular : weekdayPlural}`,
    // One line under the title: each occupied date gets its own answer.
    conflictsHint: 'Para cada día ocupado, elegí si no se reserva o si ese turno va en otra cancha abierta.',
    // A series with no end is only checked for the year ahead, the booking horizon.
    conflictsEndless: 'Como el turno fijo no tiene fecha de fin, revisamos las fechas del próximo año.',
    // The occupied dates of the court, one row each with its own selector.
    occupiedDaysTitle: (court: string) => `Días ocupados en ${court}`,
    dayChoiceLabel: 'Ese día:',
    skipDayOption: 'No reservar ese día',
    noOtherCourtFree: 'No hay otra cancha abierta ese día',
    // Above the buttons: what creating the series will do with the occupied dates.
    choicesSummary: (moved: number, skipped: number) =>
      [
        moved > 0 ? `${String(moved)} ${moved === 1 ? 'día' : 'días'} en otra cancha` : '',
        skipped > 0 ? `${String(skipped)} ${skipped === 1 ? 'día' : 'días'} sin reservar` : '',
      ]
        .filter(Boolean)
        .join(', '),
    // What holds a date that collides, as a short tag: `held_by` when the server says, else the reason.
    conflictTags: {
      booking: 'Reserva',
      fixed: 'Otro turno fijo',
      event: 'Evento',
      match: 'Partido de torneo',
      overlap: 'Ocupada',
      blocked: 'Bloqueo',
      cancelled_by_owner: 'Liberada',
      closed_day: 'Complejo cerrado',
      no_price: 'Sin precio',
      client_blocked: 'Cliente bloqueado',
      missed: 'No se pudo crear',
    },
    conflictsBack: 'Volver',
    createSuccess: (weekdayPlural: string, time: string, skipped: number) => {
      const created = `Turno fijo creado: todos los ${weekdayPlural} a las ${time}`;
      if (skipped === 0) return created;
      return `${created} · ${String(skipped)} ${skipped === 1 ? 'fecha salteada' : 'fechas salteadas'}`;
    },
    createError: 'Error al crear el turno fijo',
    // The last step's button when the booking repeats every week.
    createLabel: 'Crear turno fijo',
    // The repeat summary on the last step, one fact per row.
    summaryRepeats: 'Repite',
    summaryHours: 'Horario',
    summaryCourt: 'Cancha',
    summarySince: 'Desde',
    summaryUntil: 'Hasta',
    everyWeekday: (weekdayPlural: string) => `Todos los ${weekdayPlural}`,
    hoursRange: (from: string, to: string) => `${from} a ${to}`,
    // Indexed by weekday, 0 = Sunday, as the API sends it. Plural and lower
    // case because it follows "todos los".
    weekdaysPlural: ['domingos', 'lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábados'],
    weekdaysSingular: ['domingo', 'lunes', 'martes', 'miércoles', 'jueves', 'viernes', 'sábado'],
    // "Todos los martes a las 21:00", under a "Turno fijo" or "Repite" label.
    everyWeekdayAt: (weekdayPlural: string, time: string) => `Todos los ${weekdayPlural} a las ${time}`,
    cancelOne: 'Cancelar solo esta fecha',
    skipConfirmTitle: 'Liberar esta fecha',
    skipConfirmDescription: 'Esta fecha queda libre. El turno fijo sigue en las demás semanas.',
    skipConfirmLabel: 'Cancelar esta fecha',
    endFromHere: 'Cancelar turno fijo',
    endConfirmTitle: 'Cancelar el turno fijo',
    endConfirmDescription:
      'Esta fecha y las siguientes quedan libres. Si alguna ya tiene su reserva sin pago, se cancela; si tiene un pago, se mantiene.',
    endConfirmLabel: 'Cancelar turno fijo',
    endSuccess: (cancelled: number, kept: number) => {
      const ended = 'Turno fijo cancelado';
      const parts = [
        ...(cancelled > 0
          ? [`${String(cancelled)} ${cancelled === 1 ? 'reserva cancelada' : 'reservas canceladas'}`]
          : []),
        ...(kept > 0 ? [`${String(kept)} ${kept === 1 ? 'paga se mantiene' : 'pagas se mantienen'}`] : []),
      ];
      return [ended, ...parts].join(' · ');
    },
    skipSuccess: (dayMonth: string) => `Fecha liberada: ${dayMonth}`,
    skipError: 'Error al liberar la fecha',
    // The blocked slot of a date no booking exists for yet.
    blockLabel: (client: string) => `Turno fijo · ${client}`,
    blockTitle: 'Turno fijo',
    blockLabelCompact: 'Turno fijo',
    blockDetailTitle: 'Turno fijo',
    slotTaken: 'Ese horario es de un turno fijo. Cancelá esa fecha del turno fijo para usarlo.',
    endError: 'Error al cancelar el turno fijo',
  },
  paymentsLoadError: 'No pudimos cargar los pagos',
  paymentsLoadErrorDescription: 'Revisá tu conexión e intentá de nuevo.',
} as const;
