export const validation = {
  /**
   * Text for the error codes the API returns. The backend owns the code, this
   * owns the wording — see internal/httpx/codes.go in backend.
   */
  server: {
    slugTaken: 'Ya existe un complejo con esa URL, elegí otra',
    courtNameTaken: 'Ya existe una cancha con ese nombre, elegí otro',
    depositOver100: 'La seña no puede superar el 100%',
    depositExceedsPrice: 'La seña no puede superar el precio total',
    monthOutOfRange: 'El mes debe estar entre 1 y 12',
    reportPeriodInFuture: 'No se puede generar un reporte de un período futuro',
    reportPeriodBeforeComplex: 'El complejo no existía en ese período, no hay datos',
  },

  nameRequired: 'El nombre es requerido',
  lastNameRequired: 'El apellido es requerido',
  slugRequired: 'El slug es requerido',
  slugFormat: 'Solo letras minúsculas, números y guiones',
  addressRequired: 'Seleccioná una dirección de la lista',
  phoneRequired: 'El teléfono es requerido',
  emailRequired: 'El email es requerido',
  emailInvalid: 'Email inválido',
  phoneInvalid: 'Teléfono inválido',
  minChars8: 'Mínimo 8 caracteres',
  maxChars15: 'Máximo 15 caracteres',
  maxChars72: 'Máximo 72 caracteres',
  maxChars100: 'Máximo 100 caracteres',
  maxChars200: 'Máximo 200 caracteres',
  maxChars500: 'Máximo 500 caracteres',
  sportInvalid: 'Deporte inválido',
  courtTypeInvalid: 'Tipo inválido',
  durationInvalid: 'Duración inválida',
  priceNonNegative: 'El precio no puede ser negativo',
  dayInvalid: 'Día inválido',
  timeRequired: 'Horario requerido',
  timeRangeNotEmpty: 'El fin no puede ser igual al inicio',
  // The band-row-scoped twin of `timeRangeNotEmpty` above, not a replacement
  // for it: `blockSlotForm.schema.ts` still uses the long form in a field
  // with room for it. This one renders in `BandRow`'s own compact price
  // field slot (see `PriceField`), which at a 320px dialog is roughly 70–90px
  // wide — the 35-character original wrapped there. Kept to one line at that
  // width is the whole reason this exists as its own key.
  bandTimeRangeNotEmpty: 'Rango inválido',
  // Named for what the owner did, not for the constraint: they put two franjas
  // over the same hour, and the hour cannot have two rates. Shortened from
  // "Esta franja se superpone con otra del mismo día" (49 chars, two lines in
  // `BandRow`'s compact price field at 320px) to fit that field on one line;
  // only ever reachable from `bandsOverlap`'s own path (`bands.N.time_from`),
  // so nothing else needed the longer wording.
  bandsOverlap: 'Se superpone',
  // Only reachable once a day has a differentiated row — a day with none is
  // simply unpriced, which is a valid answer. See `dayPriceSchema`. Shortened
  // from "Poné un precio para esta franja" (32 chars) for the same reason as
  // `bandsOverlap` above — one line in the same narrow field.
  priceRequiredForBand: 'Falta el precio',
  // Only reachable once a day HAS a differentiated row: that row carves an
  // exception out of the full-day price, which then has to cover the rest of
  // the day and can no longer be left blank. See `dayPriceSchema`. Shortened
  // from "Poné un precio para el día" for the same one-line-at-320px reason,
  // even though this one renders in the day-level `PriceField` (`w-24`,
  // unshrunk) rather than `BandRow`'s compact one — kept short and distinct
  // from `priceRequiredForBand` rather than reused, since the two name
  // different things (the day's own price vs. a row's).
  priceRequiredForDay: 'Poné un precio',
  closeNotEqualOpen: 'El horario de cierre no puede ser igual al de apertura',
  selectCourt: 'Seleccioná una cancha',
  dateRequired: 'La fecha es requerida',
  timeSlotRequired: 'El horario es requerido',
  pastDate: 'No podés seleccionar una fecha pasada',
  timeInPast: 'No podés crear una reserva en un horario pasado',
  amountPositive: 'El monto debe ser mayor a 0',
  amountRequired: 'Ingresá un monto',
  amountNonNegative: 'El monto no puede ser negativo',
  cashSessionTooLarge: 'El monto es demasiado grande',
  movementAmountTooLarge: 'El monto es demasiado grande',
  // The cashbox works in whole pesos (see MoneyPesosField) — this is the
  // app's own message for a decimal amount, shown instead of letting the
  // number input's native step-mismatch silently block the submit.
  amountMustBeWhole: 'El monto debe ser en pesos enteros, sin centavos',
  selectCategory: 'Seleccioná una categoría',
  selectPaymentMethod: 'Seleccioná un método de pago',
  manualPriceRequired: 'Ingresá un precio: no hay una tarifa configurada para este horario',
  atLeastOnePrice: 'Debe haber al menos un precio',
  exactly7Days: 'Debe contener exactamente 7 días',
  min0Percent: 'Mínimo 0%',
  max100Percent: 'Máximo 100%',
  min1Hour: 'Mínimo 1 hora',
  max168Hours: 'Máximo 168 horas',
  max168HoursWeek: 'Máximo 168 horas (1 semana)',
  maxChars60: 'Máximo 60 caracteres',
  maxChars120: 'Máximo 120 caracteres',
  quantityRequired: 'Ingresá una cantidad',
  quantityPositive: 'La cantidad debe ser mayor a 0',
  quantityTooLarge: 'La cantidad es demasiado grande',
  quantityNonNegative: 'La cantidad no puede ser negativa',
  countedMustBeWhole: 'La cantidad contada debe ser un número entero',
  selectReason: 'Seleccioná un motivo',
} as const;
