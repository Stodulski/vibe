export const courts = {
  title: 'Canchas',
  pageDescription: 'Administrá las canchas de tu complejo',
  // 'Agregar' rather than 'Nueva': the owner is adding a court to a list they
  // can see, not navigating to a blank one. Same wording as the onboarding
  // step's own button, so the action is named once across the app.
  create: 'Agregar cancha',
  edit: 'Editar cancha',
  delete: 'Eliminar cancha',
  deleteConfirm: '¿Estás seguro que querés eliminar esta cancha? Esta acción no se puede deshacer.',
  name: 'Nombre',
  sport: 'Deporte',
  courtType: 'Tipo de cancha',
  active: 'Activa',
  inactive: 'Inactiva',
  prices: 'Precios',
  noPrices: 'Sin precios configurados',
  duration: 'Duración',
  dayType: 'Tipo de día',
  price: 'Precio',
  noCourts: 'Todavía no hay canchas',
  noCourtsDescription: 'Creá tu primera cancha para comenzar a recibir reservas.',
  block: 'Bloquear horario',
  blockDescription: 'Bloquea un horario para que no se pueda reservar',
  blockReason: 'Motivo (opcional)',
  blockSuccess: 'Horario bloqueado',
  blockDate: 'Fecha',
  blockStartTime: 'Hora inicio',
  blockEndTime: 'Hora fin',
  sportTypes: {
    padel: 'Pádel',
    tennis: 'Tenis',
    soccer: 'Fútbol',
    basketball: 'Básquet',
    volleyball: 'Vóley',
    hockey: 'Hockey',
    pickleball: 'Pickleball',
  },
  courtTypes: {
    indoor: 'Techada',
    outdoor: 'Descubierta',
    semi_covered: 'Semi cubierta',
  },
  dayTypes: {},
  applyToAll: 'Aplicar a todos',
  descriptionOptional: 'Descripción (opcional)',
  noDescription: 'Sin descripción',
  pricesAppliedToAll: 'Precios aplicados a todos los días',
  // Down from two paragraphs (the proportional-charging explanation and a
  // sentence about differentiated prices) to this one line: the owner does
  // not want a wall of text above a table that is otherwise self-explanatory,
  // and "Nuevo precio" already explains what it does when the owner meets it.
  pricesHourlyHint: 'Los precios son por hora.',
  // The button that adds a differentiated row under a day's full-day price —
  // quoted this way by the owner: a NEW price, not another "franja" like the
  // rows themselves are still called once they exist (see `bandOrdinal`
  // below), because this button is offering to price an exception, not to
  // grow a list.
  newPrice: 'Nuevo precio',
  removeBand: 'Eliminar franja',
  // Only ever read aloud, never drawn: it builds the accessible name that tells
  // one of a day's rows from the next ("Jueves, franja 2: desde").
  bandOrdinal: 'franja',
  bandFrom: 'Desde',
  bandTo: 'Hasta',
  // Only ever read aloud now — the visible "Día sig." marker beside a row
  // whose hours cross midnight was removed everywhere it rendered (owner
  // instruction); this is where that fact still reaches a screen reader,
  // folded into the "Hasta" select's own accessible name in `BandRow`.
  bandEndsNextDay: 'Termina al día siguiente',
  // The disclosure button's accessible name, per day and per direction —
  // seven of these sit in the dialog, so "Mostrar franjas" alone does not
  // say which day is about to open, or whether it already is.
  showBandsFor: 'Mostrar franjas de',
  hideBandsFor: 'Ocultar franjas de',
  durations: {
    60: '60 min',
    90: '90 min',
    120: '120 min',
  },
  createdSuccess: 'Cancha creada exitosamente',
  createError: 'Error al crear la cancha',
  deletedSuccess: 'Cancha eliminada',
  deleteError: 'Error al eliminar la cancha',
  pricesUpdated: 'Precios actualizados',
  pricesUpdateError: 'Error al actualizar precios',
  blockError: 'Error al bloquear el horario',
  unblockSlotSuccess: 'Horario desbloqueado',
  unblockSlotError: 'Error al desbloquear el horario',
  blockedSlots: 'Horarios bloqueados',
  unblockSlot: 'Desbloquear',
  updateSuccess: 'Cancha actualizada',
  updateError: 'Error al actualizar la cancha',
  atLeastOnePrice: 'Debe haber al menos un precio configurado',
  weekdaysShort: 'Lun - Vie',
  weekendShort: 'Sáb - Dom',
  daysShort: {
    monday: 'Lun',
    tuesday: 'Mar',
    wednesday: 'Mié',
    thursday: 'Jue',
    friday: 'Vie',
    saturday: 'Sáb',
    sunday: 'Dom',
  },
  // Column headers for the `xl` table layout. `price` above already reads
  // 'Precio', so it's reused for the general min–max range column.
  table: {
    court: 'Cancha',
    type: 'Tipo',
    description: 'Descripción',
    status: 'Estado',
  },
} as const;
