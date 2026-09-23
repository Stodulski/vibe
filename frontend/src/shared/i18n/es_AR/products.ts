export const products = {
  title: 'Productos',
  newProduct: 'Nuevo producto',
  filterActive: 'Activos',
  filterInactive: 'Inactivos',
  searchPlaceholder: 'Buscar por nombre o categoría...',
  emptyTitle: 'Todavía no cargaste productos',
  emptyDescription: 'Agregá el primer producto de tu catálogo para empezar a vender.',
  noSearchResults: 'Ningún producto coincide con la búsqueda',
  loadError: 'No pudimos cargar los productos',
  productLoadError: 'No pudimos cargar el producto',
  // ─── Badges / stock ───
  lowStockBadge: 'Stock bajo',
  needsReviewBadge: 'Revisar stock',
  noStockControl: 'Sin control de stock',
  stockLabel: 'Stock',
  priceLabel: 'Precio',
  categoryLabel: 'Categoría',
  inactiveBadge: 'Inactivo',
  // ─── Row actions ───
  editAction: 'Editar',
  restockAction: 'Reponer',
  adjustAction: 'Ajustar',
  deactivateAction: 'Desactivar',
  reactivateAction: 'Reactivar',
  // ─── Create/edit dialog ───
  createTitle: 'Nuevo producto',
  editTitle: 'Editar producto',
  nameField: 'Nombre',
  categoryField: 'Categoría',
  priceField: 'Precio',
  tracksStockField: 'Controlar stock',
  lowStockThresholdField: 'Avisar con stock bajo en',
  // Shown once a product already carries a threshold — the API cannot clear
  // it back to unset, only replace it with another number (odd/tasks/
  // pos-cashbox.md T5a: "do not invent a workaround").
  lowStockThresholdLockedHelp: 'Ya tiene un aviso configurado: se puede cambiar, pero no se puede quitar.',
  thresholdRequiredOnceSet: 'No se puede borrar una vez definido; ingresá otro valor.',
  createSuccess: 'Producto creado',
  createError: 'Error al crear el producto',
  updateSuccess: 'Producto actualizado',
  updateError: 'Error al actualizar el producto',
  // A stale `version` — someone else saved this product meanwhile. Its own
  // copy, not the generic `common.versionConflict`: the affected query here
  // is a single product, not "los datos" broadly.
  updateConflict: 'Otro cambio se guardó antes; recargamos el producto.',
  // `productsUpdate` 409 when tracks_stock is turned off with stock left —
  // the exact code path is `ErrProductHasStock` (backend/internal/products/service.go).
  stockNotZero: 'Primero ajustá el stock a cero.',
  productInactive: 'Este producto está desactivado.',
  productNotTrackingStock: 'Este producto no controla stock.',
  // ─── Restock dialog ───
  restockTitle: 'Reponer stock',
  restockQuantityField: 'Cantidad',
  restockTotalCostField: 'Costo total',
  restockMethodField: 'Método',
  restockNoteField: 'Nota',
  restockNeedsOpenTill: 'Para reponer necesitás la caja abierta: el pago sale de la caja.',
  goToCash: 'Ir a Caja',
  restockSuccess: 'Reposición registrada',
  restockError: 'Error al reponer el stock',
  // ─── Adjust dialog ───
  adjustTitle: 'Ajustar stock',
  currentStock: 'Stock actual',
  adjustCountedField: 'Cantidad contada',
  adjustDifferenceLabel: 'Diferencia',
  adjustReasonField: 'Motivo',
  adjustNoteField: 'Nota',
  adjustSuccess: 'Stock ajustado',
  adjustError: 'Error al ajustar el stock',
  reasons: {
    breakage: 'Rotura',
    expired: 'Vencimiento',
    own_consumption: 'Consumo propio',
    count_correction: 'Corrección de conteo',
    other: 'Otro',
  },
  // ─── Deactivate / reactivate confirm ───
  deactivateTitle: 'Desactivar producto',
  deactivateDescription: 'No va a aparecer para vender. Podés reactivarlo cuando quieras.',
  deactivateSuccess: 'Producto desactivado',
  deactivateError: 'Error al desactivar el producto',
  reactivateTitle: 'Reactivar producto',
  reactivateDescription: 'Va a volver a estar disponible para vender.',
  reactivateSuccess: 'Producto reactivado',
  reactivateError: 'Error al reactivar el producto',
  // ─── Detail page ───
  detailTitle: 'Detalle de producto',
  backToProducts: 'Volver a Productos',
  historyTitle: 'Movimientos de stock',
  noHistory: 'Todavía no hay movimientos de stock',
  movementKinds: {
    sale: 'Venta',
    restock: 'Reposición',
    adjustment: 'Ajuste',
    sale_void: 'Anulación de venta',
  },
} as const;
