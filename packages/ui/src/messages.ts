/**
 * Default copy for the components in this package (es_AR).
 *
 * Every user-visible string here is also a prop on the component that uses
 * it, so the app can pass its own text. These values are the ones the shared
 * components rendered before they moved out of the app, so a caller that
 * passes nothing sees the same words. Keep the list short: a string needs a
 * default here only when a component in this package renders it.
 */
export const messages = {
  loading: 'Cargando...',
  back: 'Volver',
  cancel: 'Cancelar',
  confirm: 'Confirmar',
  optional: 'opcional',
  requiredMarker: 'requerido',
  errorTitle: 'Algo salió mal',
  errorDescription: 'Ocurrió un error inesperado. Intentá de nuevo o volvé al inicio.',
  retry: 'Reintentar',
  backHome: 'Volver al inicio',
  linkExpired: 'Este link venció',
  linkExpiredDescription: 'Para consultar por tu reserva, comunicate con el complejo.',
} as const;
