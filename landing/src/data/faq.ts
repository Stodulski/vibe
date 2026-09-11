/**
 * Las preguntas frecuentes de la home, en un solo lugar.
 *
 * Estaban escritas dos veces: una en Faq.astro, que es lo que ve una persona, y
 * otra en el FAQPage de jsonld.ts, que es lo que lee Google. Hoy coinciden las
 * nueve palabra por palabra, pero eso es suerte, no una garantía: alcanza con
 * que alguien corrija una respuesta en un archivo y no en el otro.
 *
 * Y no es un problema estético. Google descuenta el structured data que
 * contradice el contenido de la página, así que una FAQ que se despega de su
 * schema no solo queda desprolija, deja de servir para lo que se puso.
 *
 * Los números salen de precio.ts y mercadopago-costos.ts. Ninguna cifra sobre
 * plata se escribe acá a mano.
 */
import { RANGO_NACIONAL } from './mercadopago-costos.ts';
import { CARGO_SERVICIO_TEXTO, CARGO_MINIMO, pesos } from './precio.ts';

export type FaqItem = {
  question: string;
  answer: string;
  /** Clase de animación que ya tenía cada bloque en la home. */
  anim: string;
};

/* La frase del cargo aparece en dos respuestas, y tiene que decir lo mismo en
   las dos o la segunda parece una condición distinta. */
const CARGO = `un cargo de servicio del ${CARGO_SERVICIO_TEXTO} sobre la seña, `
  + `no sobre el precio total de la cancha, con un mínimo de ${pesos(CARGO_MINIMO)}`;

export const faq: FaqItem[] = [
  {
    anim: 'anim-d1',
    question: '¿Vibe es realmente gratis? ¿Cuál es el truco?',
    answer: `Para el complejo sí: $0 de costo fijo, sin suscripción. De tu seña solo se descuenta `
      + `la comisión de MercadoPago (${RANGO_NACIONAL} más IVA, según tu provincia y cuándo elijas `
      + `acreditar el dinero). Vibe se cobra del lado del cliente: al reservar se le suma un cargo de servicio del ${CARGO_SERVICIO_TEXTO} sobre la seña, con un mínimo de ${pesos(CARGO_MINIMO)}. `
      + `Ese es el modelo completo, no hay nada más.`,
  },
  {
    anim: 'anim-d9',
    question: '¿Mis clientes pagan algo extra por reservar?',
    answer: `Sí, y conviene decirlo de frente: al reservar se les suma ${CARGO}. Es lo único que `
      + `cobra Vibe y es lo que hace que vos no pagues nada. Si el cliente cancela dentro del plazo `
      + `que definiste, se le reembolsa junto con la seña.`,
  },
  {
    anim: 'anim-d2',
    question: '¿Mis clientes necesitan descargarse una app?',
    answer: 'No. Tus clientes reservan desde el celular entrando a tu link, sin registro obligatorio '
      + 'ni descarga de app. Funciona directo desde el navegador.',
  },
  {
    anim: 'anim-d3',
    question: '¿Qué pasa si un cliente cancela la reserva?',
    answer: 'Si cancelás o el cliente cancela dentro del plazo permitido, el reembolso de la seña se '
      + 'procesa automáticamente por MercadoPago. Sin intervención tuya.',
  },
  {
    anim: 'anim-d4',
    question: '¿Puedo seguir tomando reservas por teléfono o en persona?',
    answer: 'Sí. Desde el panel podés crear reservas manuales para clientes que prefieran reservar '
      + 'por otro medio. Todo queda centralizado en un solo lugar.',
  },
  {
    anim: 'anim-d5',
    question: '¿Cómo recibo la plata de las señas?',
    answer: 'Las señas se cobran a través de MercadoPago y se acreditan directo en tu cuenta. No '
      + 'manejamos tu dinero en ningún momento.',
  },
  {
    anim: 'anim-d6',
    question: '¿Cuánto tarda configurar mi complejo?',
    answer: 'Menos de 5 minutos. Creás tu cuenta en app.vibe.com.ar/register, cargás tus canchas, '
      + 'horarios y precios, y conectás MercadoPago. Podés recibir reservas el mismo día.',
  },
  {
    anim: 'anim-d7',
    question: '¿Puedo poner precios diferentes según el día y horario?',
    answer: 'Sí. Podés configurar precio distinto por cancha, día de la semana y franja horaria. Hora '
      + 'pico más cara, mañanas más baratas. Turnos de 60, 90 o 120 minutos.',
  },
  {
    anim: 'anim-d8',
    question: '¿Qué pasa con los clientes que faltan sin avisar?',
    answer: 'Como cobrás seña al reservar, reducís las ausencias. Y antes del turno les llegan recordatorios '
      + 'automáticos por WhatsApp y email. Si alguien reincide, podés ver su '
      + 'historial y bloquearlo con un click.',
  },
];
