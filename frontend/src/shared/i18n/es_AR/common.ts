export const common = {
  save: 'Guardar',
  saving: 'Guardando...',
  cancel: 'Cancelar',
  delete: 'Eliminar',
  edit: 'Editar',
  rowActionsLabel: 'Más acciones',
  create: 'Crear',
  search: 'Buscar...',
  loading: 'Cargando...',
  noResults: 'No se encontraron resultados',
  confirm: 'Confirmar',
  back: 'Volver',
  next: 'Siguiente',
  close: 'Cerrar',
  no: 'No',
  error: 'Error',
  success: 'Operación exitosa',
  required: 'Campo requerido',
  actions: 'Acciones',
  filters: 'Filtros',
  export: 'Exportar',
  refresh: 'Actualizar',
  updateAvailable: 'Hay una versión nueva de Vibe.',
  updateNow: 'Actualizar',
  optional: 'opcional',
  requiredMarker: 'requerido',
  requiredHint: 'Los campos obligatorios están marcados con *',
  all: 'Todos',
  complex: 'Complejo',
  day: 'Día',
  change: 'Cambiar',
  tryAnotherSearch: 'Probá con otro término de búsqueda',
  // Generic query-error copy for any data view: rendered before the empty
  // state so an API failure never shows a false "no data" screen.
  loadError: 'No pudimos cargar los datos.',
  loadErrorDescription: 'Revisá tu conexión e intentá de nuevo.',
  // Shown when the server's response doesn't match the shape the app
  // expects (an ApiResponseError), instead of a mutation-specific fallback
  // that would misattribute a shape mismatch to the action itself.
  invalidResponse: 'La respuesta del servidor no es válida. Probá de nuevo.',
  // Shown for a timeout, a dropped connection, or the browser knowing it's
  // offline — never for a server-side error, which keeps its own specific
  // copy (see getHttpErrorMessage in src/shared/lib/utils.ts).
  networkError: 'No pudimos conectarnos. Revisá tu conexión e intentá de nuevo.',
} as const;
