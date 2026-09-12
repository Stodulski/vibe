export const publicBooking = {
  stepProgress: 'Progreso de reserva',
  stepSelect: 'Turno',
  stepData: 'Datos',
  stepPay: 'Pago',
  slotNoLongerAvailable: 'Ese turno ya no está disponible',
  selectCourt: 'Elegí la cancha',
  confirmBooking: 'Confirmar reserva',
  bookingSuccess: 'Reserva confirmada',
  bookingPending: 'Reserva pendiente de pago',
  noAvailability: 'Sin horarios para esta fecha',
  today: 'Hoy',
  closed: 'Cerrado',
  // A complex that has not connected MercadoPago cannot take a booking
  // online. Its page used to render the whole availability grid with every
  // slot disabled — hundreds of dead buttons — above a warning that read as
  // if the site were broken. It does take bookings, by phone; that is what
  // the page says now.
  phoneOnlyTitle: 'Reservá por WhatsApp',
  phoneOnlyDescription: 'Este complejo todavía no toma reservas online. Escribile para reservar.',
  phoneOnlyCallLabel: 'Escribir al complejo por WhatsApp',
  phoneOnlyMessage: 'Hola, quiero reservar una cancha.',
  available: 'Disponible',
  occupied: 'Ocupado',
  continue: 'Continuar',
  firstName: 'Nombre',
  lastName: 'Apellido',
  phone: 'Teléfono',
  phoneHelper: 'Sin 0 ni 15. Ej: 1123456789',
  email: 'Email',
  emailPlaceholder: 'tu@email.com',
  notes: 'Notas',
  notesPlaceholder: 'Comentario para el complejo',
  optional: 'opcional',
  total: 'Total',
  courtPrice: 'Precio del turno',
  totalOnline: 'Pagás ahora',
  deposit: 'Seña',
  remaining: 'Resta pagar',
  // The button's own label, with no amount concatenated onto it — that
  // concatenation (`"Pagar · $1.010"`) was what forced it onto two lines.
  // The amount lives beside the button instead (`SubmitFooter`), reusing
  // `totalOnline` below, so the deposit-plus-service-fee total the
  // breakdown already explains is still visible right where the visitor
  // commits to it, just not baked into the button's own text.
  paySecurely: 'Pagar de manera segura',
  // Suffixes appended to a slot button's accessible name, after the time and
  // the price. They start with a comma because a screen reader reads the
  // whole label as one sentence.
  slotSuffixUnavailable: ', no disponible',
  slotSuffixSelected: ', seleccionado',
  cancellationPolicyBefore: 'Podés cancelar hasta',
  cancellationPolicyAfter: 'antes del turno',
  // Joins the start and end time on the success page's summary line: "19:00
  // a 20:00".
  timeRangeTo: 'a',
  redirectingToMP: 'Abriendo MercadoPago...',
  processingPayment: 'Procesando pago...',
  // The landing after a rejected payment. This used to be a toast on the
  // complex page — a message with a countdown, over the full grid, on the
  // worst news of the flow. It is a page now, so these read as a page.
  paymentFailed: 'Pago rechazado',
  paymentFailedDescription: 'No se cobró nada. El turno quedó libre.',
  paymentFailedSlotLabel: 'Turno',
  paymentFailedChooseAnother: 'Elegir otro horario',
  paymentFailedCallComplex: 'Llamar al complejo',
  paymentProcessing: 'Pago en proceso',
  paymentProcessingDescription: 'Puede demorar unos minutos. Te avisamos por WhatsApp.',
  paymentUnderReview: 'Pago en revisión',
  paymentUnderReviewDescription: 'MercadoPago lo está revisando. Puede demorar. Te avisamos por WhatsApp.',
  // Shown instead of the "procesando"/"pago en proceso" copy when the
  // status check itself fails (a network drop or a 500), not the payment.
  // The booking may well be fine — this asks to check again rather than
  // sending the person off to book a second time, which could double-book
  // someone whose payment already went through.
  paymentStatusError: 'No pudimos consultar el estado de tu pago',
  paymentStatusErrorDescription: 'Hubo un problema de conexión. Probá de nuevo.',
  bookingRef: 'Reserva',
  tryAgain: 'Reintentar',
  makeAnother: 'Nueva reserva',
  schedule: 'Horario',
  address: 'Dirección',
  // Shown instead of the map widget when it fails to load (a bad tile
  // server, a bundling issue) — the rest of the club's page still works, so
  // this replaces only the map, not the whole page.
  mapUnavailable: 'No pudimos cargar el mapa',
  complexNotFound: 'Complejo no encontrado',
  complexNotFoundDescription: 'No existe o fue eliminado.',
  // Shown instead of complexNotFoundDescription when the URL carries a slug
  // (the common case: a stale WhatsApp link or bookmark to a complex that
  // was renamed or deleted) — naming the venue is more useful than the
  // generic sentence above. Split around the slug the same way
  // cancellationPolicyBefore/After splits around the hour count.
  complexNotFoundSlugPrefix: 'No encontramos el complejo',
  complexNotFoundSlugSuffix: '. El enlace puede haber cambiado o el complejo ya no está disponible.',
  // Shown instead of complexNotFound when useComplexBySlug fails with
  // something other than a 404 (a 500 or no connection): the complex might
  // exist, so the message says to try again instead of that it's gone.
  complexLoadError: 'No pudimos cargar el complejo',
  complexLoadErrorDescription: 'Hubo un problema de conexión. Probá de nuevo.',
  // Shown inline in the availability grid when useAvailability fails, so a
  // failed request never reads as "closed" (t.publicBooking.closed).
  availabilityLoadError: 'No pudimos cargar los horarios disponibles.',
  // Shown instead of invalidCancelLink when GET /book/cancel-info fails
  // with something other than 404/410.
  cancelInfoLoadError: 'No pudimos cargar tu reserva',
  cancelInfoLoadErrorDescription: 'Hubo un problema de conexión. Probá de nuevo.',
  selectDate: 'Elegí la fecha',
  dateStripLabel: 'Fecha',
  indoor: 'Techada',
  outdoor: 'Descubierta',
  semi_covered: 'Semi cubierta',
  summary: 'Tu reserva',
  slots: 'Turnos',
  slot: 'turno',
  availableSlots: 'horarios disponibles',
  // Shown on a time when its free courts disagree on price — `court_prices`
  // hangs off the court, so a venue may charge more for the covered one.
  priceFrom: 'desde',
  // On the duration buttons. "90" alone is what a screen reader announces
  // and what stands next to a price once the heading scrolls away.
  minutesShort: 'min',
  // The guided steps. Questions, not section titles: a step asks something,
  // and the answer is what the breadcrumb above it shows afterwards.
  stepSportQuestion: '¿Qué vas a jugar?',
  stepDurationQuestion: '¿Cuánto tiempo?',
  stepSportLabel: 'Deporte',
  stepDurationLabel: 'Duración',
  stepChange: 'Cambiar',
  // The mobile-only "Paso 2 de 3: Tus datos" line under StepIndicator's
  // circles — split around the numbers rather than templated, the same way
  // cancellationPolicyBefore/After splits around the hour count above.
  stepOfPrefix: 'Paso',
  stepOfMiddle: 'de',
  // The contact block keeps its shape whether or not there is an email, so
  // the gap is named rather than left as a missing line nobody can tell was
  // ever meant to be there. Services get the opposite treatment: an empty
  // list drops its whole disclosure, because an accordion that opens onto
  // "nothing here" is a control that wastes the one tap it asks for.
  noEmail: 'Sin email',
  lastCourtLeft: 'Última cancha',
  noAvailableSlots: 'Sin disponibilidad',
  checkingAvailability: 'Buscando horarios...',
  slotConflict: 'El horario ya no está disponible',
  accountBlocked: 'Tu cuenta está bloqueada. Contactá al complejo.',
  bookingCreateError: 'No se pudo crear la reserva',
  // A 503 from POST /book (internal/bookings/public.go): the booking itself
  // was created but MercadoPago's preference could not be, so the server
  // cancels it and the slot is free again. A toast that disappears left
  // people re-typing everything with no idea what happened; this stays on
  // screen until they act, and says the data survived the failed attempt.
  paymentLinkError: 'No se pudo generar el link de pago. Tus datos siguen cargados.',
  retryPaymentLink: 'Reintentar',
  generatingPaymentLink: 'Generando el link de pago',
  // Shown on the success page when polling lands on `status: "cancelled"`
  // with a `payment_status` other than "unpaid" (`internal/payments/process.go`'s
  // processRejectedPayment never changes payment_status — it has no
  // "rejected" value — so "unpaid" is the one shape a genuinely declined
  // payment can leave). Any other payment_status means money moved and is
  // being reversed, which is a different story than a rejected card.
  bookingCancelled: 'Tu reserva fue cancelada',
  bookingCancelledDescription: 'Si pagaste algo, te lo devolvemos al mismo medio.',
  cancelBooking: 'Cancelar reserva',
  cancelBookingDescription: 'La seña se devuelve automáticamente',
  refundManualDescription: 'El complejo te devuelve la seña. Coordinalo con ellos.',
  noPaymentToRefund: 'No hay pagos para devolver.',
  cancelBookingConfirm: 'Confirmar cancelación',
  cancelBookingSuccess: 'Reserva cancelada',
  cancelledSuccessfully: 'No hace falta que hagas nada más.',
  cancelBookingError: 'No se pudo cancelar la reserva',
  bookingNotFound: 'Reserva no encontrada',
  invalidCancelLink: 'El link no es válido.',
  linkNotFoundDescription: 'El link no es válido o venció.',
  linkExpired: 'Este link venció',
  linkExpiredDescription: 'Para consultar por tu reserva, comunicate con el complejo.',
  alreadyCancelled: 'Esta reserva ya fue cancelada',
  alreadyCompleted: 'Esta reserva ya fue completada',
  refundExpired: 'Venció el plazo para cancelar con devolución',
  refundExpiredBefore: 'Las cancelaciones con devolución se hacen con al menos',
  refundExpiredAfter: 'de anticipación. Si cancelás ahora,',
  noRefundWarning: 'no se devuelve la seña',
  cancelNoRefund: 'Cancelar sin devolución',
  noRefundTitle: 'Sin devolución de la seña',
  cancelNoRefundDetail: 'Al confirmar, la reserva se cancela y',
  noMoneyBack: 'la seña no se devuelve',
  confirmCancelNoRefund: 'Cancelar sin devolución',
  confirmCancelTitle: '¿Cancelar la reserva?',
  confirmCancelRefundDetail: 'La seña se devuelve automáticamente.',
  confirmCancelRefundManualDetail: 'El complejo te devuelve la seña a mano.',
  confirmCancelYes: 'Sí, cancelar',
  allSports: 'Todos',
  morning: 'Mañana',
  afternoon: 'Tarde',
  evening: 'Noche',
  bookYourCourt: 'Reserva tu cancha',
  bookCourtsAt: 'Reserva canchas en',
  bookCourtsAtSuffix: '. Rápido y seguro.',
  changeTimeSlot: 'Cambiar horario',
  // The success page's money summary: two rows, label left and amount right.
  // `paidLabel` is deposit plus service fee summed (the split was shown
  // before paying and a refund returns both); `oweAtClubLabel` is the
  // balance for the venue, 0 when nothing is left.
  paidLabel: 'Pagaste',
  oweAtClubLabel: 'Resta pagar',
  // The three cancellation-window sentences on the success page, driven by
  // `cancellation` from GET /book/status. The first splits around the
  // formatted deadline the same way cancellationPolicyBefore/After does
  // around the hour count.
  cancelRefundUntilPrefix: 'Cancelación gratis hasta el',
  cancelRefundUntilSuffix: '.',
  cancelRefundUntilTurnStart: 'Cancelación gratis hasta que empiece el turno.',
  cancelRefundWindowPassed: 'Ya no podés cancelar gratis.',
  // The cancel page's subtitle, once `GET /book/cancel-info` answers with
  // `refund_amount`/`paid_amount` — same split-around-the-amount shape as
  // paidPrefix/paidDepositAndFeeSuffix above. `refundAmount` already
  // includes the service fee, so this never says the fee is excluded.
  cancelBookingQuestion: '¿Cancelás la reserva?',
  refundRowLabel: 'Te devolvemos',
  paidAmountNoRefundSuffix: 'Si cancelás ahora, no se devuelve.',
} as const;
