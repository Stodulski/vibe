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
  // Cash handed back by hand for a cancelled booking, subtracted from
  // expected cash — shown only when there was at least one during the
  // session's window (cash-manual-refunds).
  cashManualRefunds: 'Devoluciones en efectivo',
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
    // System categories: written by the sales and restock flows, never
    // offered on the manual movement form.
    sale: 'Venta',
    restock: 'Reposición',
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
  // ─── Vender (pos-cashbox T5b) ───
  sellTitle: 'Vender',
  sellNeedsOpenTill: 'Abrí la caja para vender',
  sellGoToShift: 'Ir a Turno',
  sellLoadError: 'No pudimos cargar el catálogo',
  sellSearchPlaceholder: 'Buscar por nombre...',
  sellAllCategories: 'Todas',
  sellNoResults: 'Ningún producto coincide con la búsqueda',
  sellEmptyCatalog: 'Todavía no cargaste productos activos',
  sellOutOfStock: 'Sin stock',
  sellStockLeftPrefix: 'Quedan',
  cartTitle: 'Carrito',
  cartEmpty: 'El carrito está vacío. Tocá un producto para agregarlo.',
  cartRemove: 'Quitar',
  cartDecreaseAction: 'Restar',
  cartIncreaseAction: 'Sumar',
  cartMaxLinesReached: 'Llegaste al máximo de 50 productos distintos en un carrito.',
  cartLineStockWarning: 'Sin stock suficiente: se vende igual',
  cartTotal: 'Total',
  cartMethod: 'Método',
  cartNote: 'Nota',
  cartViewAction: 'Ver carrito',
  cartChargeActionPrefix: 'Cobrar',
  cartChargeSuccess: 'Venta registrada',
  cartChargeError: 'Error al registrar la venta',
  cartDroppedStaleLines: 'Sacamos del carrito productos que ya no están activos.',
  saleConfirmationTitle: 'Venta registrada',
  saleConfirmationTotal: 'Total cobrado',
  saleConfirmationMethod: 'Método',
  saleConfirmationStockWarningTitle: 'Revisar stock',
  saleConfirmationDone: 'Listo',
  saleItemNotSellable: 'Este producto ya no está disponible para vender.',
  saleAlreadyVoided: 'Esta venta ya fue anulada.',
  // ─── Session sales list ───
  salesTitle: 'Ventas de este turno',
  noSales: 'Todavía no se registraron ventas en este turno',
  saleVoidAction: 'Anular',
  saleVoidedBadge: 'Anulada',
  saleVoidConfirmTitle: 'Anular venta',
  saleVoidConfirmDescription: 'Se va a restaurar el stock vendido y anular el ingreso de esta venta.',
  saleVoidNotePlaceholder: 'Motivo de la anulación (opcional)...',
  saleVoidSuccess: 'Venta anulada',
  saleVoidError: 'Error al anular la venta',
} as const;
