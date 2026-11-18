export const navigation = {
  dashboard: 'Dashboard',
  bookings: 'Reservas',
  cash: 'Caja',
  events: 'Eventos',
  courts: 'Canchas',
  clients: 'Clientes',
  reports: 'Reportes',
  socialKit: 'Kit para redes',
  settings: 'Configuración',
  profile: 'Mi perfil',
  account: 'Mi cuenta',
  accountSettings: 'Ajustes',
  userManual: 'Manual de uso',
  // The social kit page (`/kit`): its intro, the two pieces and the two formats.
  socialKitIntro: 'Descargá estas imágenes y subilas a tus historias y posts. Llevan el link y el QR de tu complejo.',
  socialKitPieces: {
    reservar: 'Link de reservas',
    'como-reservar': 'Cómo reservar',
  },
  socialKitFormats: {
    historia: 'Historia',
    post: 'Post',
  },
  socialKitDownload: 'Descargar',
  socialKitFormatGroup: 'Formato',
  socialKitFormatHints: {
    historia: 'Para historias de Instagram y estados de WhatsApp. 1080 × 1920.',
    post: 'Para el feed de Instagram y Facebook. 1080 × 1440.',
  },
} as const;
