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
  todayRevenue: 'Ingresos de hoy',
  todayIncomeBookings: 'Turnos',
  todayIncomeBar: 'Bar',
  todayIncomeOther: 'Otros',
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
  // "Completá tu perfil" checklist: what the public page is missing. Each
  // `why` says what the owner gains, in one line.
  profileChecklist: {
    title: 'Completá tu perfil público',
    intro: 'Un perfil completo se ve mejor en Google y ayuda a que más jugadores reserven.',
    progress: (done: number, total: number) => `${String(done)} de ${String(total)} listos`,
    items: {
      cover: {
        label: 'Foto de portada',
        why: 'Las fotos hacen que más jugadores elijan tu complejo.',
        action: 'Subí la foto de portada',
      },
      logo: {
        label: 'Logo',
        why: 'Tu logo ayuda a que te reconozcan en Google y en la página de tu complejo.',
        action: 'Subí tu logo',
      },
      description: {
        label: 'Descripción',
        why: 'Una descripción clara le cuenta a Google y a los jugadores cómo es tu complejo.',
        action: 'Escribí tu descripción',
      },
      amenities: {
        label: 'Servicios',
        why: 'Mostrar los servicios ayuda a que te encuentren quienes buscan algo puntual.',
        action: 'Elegí tus servicios',
      },
      location: {
        label: 'Ubicación en el mapa',
        why: 'Con tu ubicación en el mapa, los jugadores llegan a tu complejo sin perderse.',
        action: 'Marcá tu ubicación',
      },
      hours: {
        label: 'Revisá tus horarios',
        why: 'Confirmá que los días y horas reflejan cómo abrís hoy.',
        action: 'Editá los horarios',
      },
      phone: {
        label: 'Teléfono de contacto',
        why: 'Un teléfono visible genera confianza a la hora de reservar.',
        action: 'Cargá tu teléfono',
      },
      pricedCourt: {
        label: 'Cancha con precio',
        why: 'Con una cancha y su precio cargados, los jugadores pueden reservar en el momento.',
        action: 'Cargá los precios',
      },
    },
  },
  exportGenericError: 'No se pudo descargar el archivo. Intentá de nuevo.',
  exportTooLarge: 'El período elegido tiene demasiados pagos para exportar. Elegí un rango más corto.',
  exportTimedOut: 'La exportación tardó demasiado y no se pudo completar. Intentá de nuevo en unos minutos.',
  // JOB-06: the async export job's own copy. `exportGenerating` is the
  // button label while a job is pending/running (distinct from
  // `reports.downloading`, the sync fallback's label); `exportExpired`
  // covers the 410 the status route answers once the file's 24h retention
  // has passed.
  exportGenerating: 'Generando el archivo…',
  exportExpired: 'El archivo generado ya venció. Volvé a exportarlo.',
  realtimeAccessEnded:
    'Se dejaron de recibir actualizaciones en vivo de este complejo. Recargá la página si creés que es un error.',
  // ─── Low-stock alert (pos-cashbox T6) ───
  lowStockTitle: 'Stock bajo',
  lowStockViewProducts: 'Ver productos',
} as const;
