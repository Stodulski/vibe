/**
 * Las preguntas frecuentes de la home, en un solo lugar.
 *
 * Estaban escritas dos veces: una en Faq.astro, que es lo que ve una persona, y
 * otra en el FAQPage de jsonld.ts, que es lo que lee Google. Hoy coinciden las
 * seis palabra por palabra, pero eso es suerte, no una garantía: alcanza con
 * que alguien corrija una respuesta en un archivo y no en el otro.
 *
 * Y no es un problema estético. Google descuenta el structured data que
 * contradice el contenido de la página, así que una FAQ que se despega de su
 * schema no solo queda desprolija, deja de servir para lo que se puso.
 *
 * Los números salen de precio.ts. Ninguna cifra sobre plata se escribe acá a
 * mano.
 */

export type FaqItem = {
  question: string;
  answer: string;
  /** Clase de animación que ya tenía cada bloque en la home. */
  anim: string;
};

export const faq: FaqItem[] = [
  {
    anim: 'anim-d1',
    question: '¿Mis clientes necesitan descargarse una app?',
    answer: 'No. Tus clientes reservan desde el celular entrando a tu link, sin registro obligatorio '
      + 'ni descarga de app. Funciona directo desde el navegador.',
  },
  {
    anim: 'anim-d2',
    question: '¿Qué pasa si un cliente cancela la reserva?',
    answer: 'Si cancela dentro del plazo permitido, MercadoPago procesa el reembolso de la seña '
      + 'automáticamente, sin intervención tuya.',
  },
  {
    anim: 'anim-d3',
    question: '¿Cómo recibo la plata de las señas?',
    answer: 'Las señas se cobran con MercadoPago y se acreditan directo en tu cuenta. Vibe no maneja '
      + 'tu dinero en ningún momento.',
  },
  {
    anim: 'anim-d4',
    question: '¿Cuánto tarda configurar mi complejo?',
    answer: 'Menos de 5 minutos: creás tu cuenta, cargás canchas, horarios y precios, y conectás '
      + 'MercadoPago. Podés recibir reservas el mismo día.',
  },
  {
    anim: 'anim-d5',
    question: '¿Puedo poner precios diferentes según el día y horario?',
    answer: 'Sí. Precio distinto por cancha, día y franja horaria, con turnos de 60, 90 o 120 minutos.',
  },
  {
    anim: 'anim-d6',
    question: '¿Qué pasa con los clientes que faltan sin avisar?',
    answer: 'Cobrar seña al reservar reduce las ausencias, y los recordatorios automáticos por WhatsApp '
      + 'y email bajan las faltas todavía más.',
  },
];
