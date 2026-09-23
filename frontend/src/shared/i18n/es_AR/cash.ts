export const cash = {
  title: 'Caja',
  // ─── Empty / closed state ───
  closedTitle: 'La caja está cerrada',
  closedDescription: 'Abrí la caja para empezar a registrar ingresos, egresos y cobros en efectivo.',
  openAction: 'Abrir caja',
  // ─── Open-session dialog ───
  openingCash: 'Monto inicial',
  openingNote: 'Nota',
  openSuccess: 'Caja abierta',
  openError: 'Error al abrir la caja',
  // ─── Open session header / summary ───
  openedAt: 'Abierta',
  openingCashLabel: 'Efectivo inicial',
  expectedCash: 'Efectivo esperado',
  incomeTotal: 'Ingresos',
  expenseTotal: 'Egresos',
  bookingPaymentsTitle: 'Cobros de reservas',
  bookingPaymentsOnlineNote:
    'Solo el efectivo forma parte del efectivo esperado; el resto se muestra a modo informativo.',
  informational: 'Informativo',
  // ─── Actions ───
  incomeAction: 'Ingreso',
  expenseAction: 'Egreso',
  closeAction: 'Cerrar caja',
  // ─── Movement form dialog ───
  movementCategory: 'Categoría',
  movementMethod: 'Método',
  movementAmount: 'Monto',
  movementNote: 'Nota',
  incomeSuccess: 'Ingreso registrado',
  expenseSuccess: 'Egreso registrado',
  movementError: 'Error al registrar el movimiento',
  // ─── Movement list ───
  movementsTitle: 'Movimientos',
  noMovements: 'Todavía no hay movimientos en esta caja',
  voidAction: 'Anular',
  voidConfirmTitle: 'Anular movimiento',
  voidConfirmDescription:
    'Se va a registrar un movimiento inverso por el mismo monto. Esta acción no se puede deshacer.',
  voidNotePlaceholder: 'Motivo de la anulación (opcional)...',
  voidedBadge: 'Anulado',
  voidOfPrefix: 'Anulación de',
  // Composed after `voidOfPrefix` ("Anulación de ...") when a void's original
  // movement isn't in the currently-loaded list. Its own key instead of
  // reusing `movementsTitle` ("Movimientos"), which read as the nonsensical
  // "Anulación de Movimientos" for one row.
  voidOfUnknownMovement: 'un movimiento',
  voidSuccess: 'Movimiento anulado',
  voidError: 'Error al anular el movimiento',
  categories: {
    other_income: 'Otros ingresos',
    supplies: 'Insumos',
    salaries: 'Sueldos',
    services: 'Servicios',
    maintenance: 'Mantenimiento',
    cleaning: 'Limpieza',
    withdrawal: 'Retiro',
    other_expense: 'Otros egresos',
  },
  kinds: {
    income: 'Ingreso',
    expense: 'Egreso',
  },
  // ─── Close dialog ───
  closeTitle: 'Cerrar caja',
  closeExpectedHint: 'Efectivo esperado',
  countedCash: 'Efectivo contado',
  closeNote: 'Nota',
  closeConfirmWarning: 'Una vez cerrada, esta caja no se puede volver a abrir.',
  closeSuccess: 'Caja cerrada',
  closeError: 'Error al cerrar la caja',
  difference: 'Diferencia',
  surplus: 'Sobrante',
  shortfall: 'Faltante',
  noDifference: 'Sin diferencia',
  // ─── History ───
  historyTitle: 'Historial de cajas',
  noHistory: 'Todavía no se cerró ninguna caja',
  sessionDetailTitle: 'Detalle de caja',
  backToCash: 'Volver a Caja',
  countedCashLabel: 'Contado',
  closedAt: 'Cerrada',
  loadError: 'No pudimos cargar la caja',
  // ─── Section tabs (pos-cashbox T5a) ───
  // "Turno" names the existing `/cash` screen from here on — the page's own
  // `PageHeader` keeps the "Caja" title, this is only the tab label, next to
  // "Vender" (T5b, a placeholder until then) and "Productos".
  tabs: {
    turno: 'Turno',
    vender: 'Vender',
    productos: 'Productos',
  },
  sellComingSoonTitle: 'Vender',
  sellComingSoonDescription: 'Esta pantalla todavía no está lista. Muy pronto vas a poder vender desde acá.',
} as const;
