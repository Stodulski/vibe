export const dashboard = {
  title: 'Dashboard',
  bookingsToday: 'Reservas hoy',
  totalClients: 'Clientes totales',
  occupancyRate: 'Ocupación',
  revenue: 'Ingresos',
  occupancy: 'Ocupación por horario',
  occupancyHeatmapLabel: 'Mapa de calor de ocupación por hora y día',
  // What a ComparisonBadge is measured against, said out loud for anyone
  // reading the card rather than looking at it.
  versusYesterday: 'ayer',
  versusLastMonth: 'el mes pasado',
  revenueChartWeekLabel: 'Gráfico de ingresos: última semana',
  revenueChartMonthLabel: 'Gráfico de ingresos: último mes',
  revenuePeriodLabel: 'Período de ingresos',
  reportLoadError: 'Error al cargar el reporte',
  reportLoadErrorDescription: 'No se pudieron obtener los datos. Probá de nuevo.',
  reportNoData: 'Sin datos',
  reportNoDataDescription: 'No hay pagos registrados para este período.',
  upcomingBookings: 'Próximas reservas',
  viewAll: 'Ver todas',
  noUpcoming: 'No hay reservas próximas para hoy',
  newBooking: 'Crear reserva',
  trends: 'Tendencias',
  week: 'Semana',
  month: 'Mes',
  cashControl: 'Control de caja',
  paymentStatus: 'Estado de cobro',
  noBookingsToday: 'Sin reservas hoy',
  paymentMethodLabel: 'Método de pago',
  // Keyed by the booking's collection_status, which is what
  // GetPaymentSummary groups by since backend split payment_status in two. The two
  // refund labels this map used to carry are gone with the grouping: this
  // card sums payments that are NOT refunded over bookings that are NOT
  // cancelled, so a refund bucket here only ever held collected money filed
  // under a refund name.
  paymentStatusLabels: {
    fully_paid: 'Pagado',
    deposit_paid: 'Señado',
    unpaid: 'Pendiente',
  },
  clientsTitle: 'Clientes',
  newClients: 'Nuevos',
  recurringClients: 'Recurrentes',
  topLoyal: 'Top fieles (30 días)',
  noClientData: 'No hay datos de clientes todavía',
  noTopClients: 'Todavía no hay clientes frecuentes',
  increase: 'Aumento',
  decrease: 'Disminución',
  copyLink: 'Copiar enlace',
  linkCopied: 'Enlace copiado al portapapeles',
  exportGenericError: 'No se pudo descargar el archivo. Intentá de nuevo.',
  exportTooLarge: 'El período elegido tiene demasiados pagos para exportar. Elegí un rango más corto.',
  exportTimedOut: 'La exportación tardó demasiado y no se pudo completar. Intentá de nuevo en unos minutos.',
  realtimeAccessEnded:
    'Se dejaron de recibir actualizaciones en vivo de este complejo. Recargá la página si creés que es un error.',
} as const;
