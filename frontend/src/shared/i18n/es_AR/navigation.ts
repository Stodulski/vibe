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
  // The social kit page (`/kit`): its intro, the two pieces, the two formats and the download buttons.
  socialKitIntro: 'Descargá estas imágenes y subilas a tus historias y posts. Llevan el link y el QR de tu complejo.',
  socialKitPieces: {
    reservar: 'Link de reservas',
    'como-reservar': 'Cómo reservar',
  },
  socialKitPieceDescriptions: {
    reservar: 'El QR y el link para que tus clientes reserven desde el celular.',
    'como-reservar': 'Los tres pasos para reservar en tu complejo, con el QR.',
  },
  socialKitFormats: {
    historia: 'Historia',
    post: 'Post',
  },
  socialKitDownloads: {
    historia: 'Descargar historia',
    post: 'Descargar post',
  },
} as const;
