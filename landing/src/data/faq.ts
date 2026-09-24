/**
 * Las preguntas frecuentes de la home, en un solo lugar.
 *
 * Estaban escritas dos veces: una en Faq.astro, que es lo que ve una persona, y
 * otra en el FAQPage de jsonld.ts, que es lo que lee Google. Hoy coinciden las
 * ocho palabra por palabra, pero eso es suerte, no una garantía: alcanza con
 * que alguien corrija una respuesta en un archivo y no en el otro.
 *
 * Y no es un problema estético. Google descuenta el structured data que
 * contradice el contenido de la página, así que una FAQ que se despega de su
 * schema no solo queda desprolija, deja de servir para lo que se puso.
 *
 * Los números salen de precio.ts. Ninguna cifra sobre plata se escribe acá a
 * mano.
 */
import { CARGO_SERVICIO_TEXTO, CARGO_MINIMO, pesos } from './precio.ts';

export type FaqItem = {
  question: string;
  answer: string;
  /** Clase de animación que ya tenía cada bloque en la home. */
  anim: string;
};

export const faq: FaqItem[] = [
  {
    anim: 'anim-d1',
    question: '¿Vibe es gratis de verdad?',
    answer: 'Sí, para tu complejo. Vibe no te cobra abono, ni costo fijo, ni nada sobre lo que cobrás '
      + 'en el mostrador o vendés en el bar. Nunca. Hay dos costos, y conviene tenerlos claros: el que '
      + `reserva online paga un cargo de servicio del ${CARGO_SERVICIO_TEXTO} sobre la seña, con un mínimo `
      + `de ${pesos(CARGO_MINIMO)}; y MercadoPago descuenta su comisión del pago online que entra a tu `
      + 'cuenta, como con cualquier sistema que cobre por MercadoPago.',
  },
  {
    anim: 'anim-d2',
    question: '¿Vibe es solo para reservas?',
    answer: 'No. Reservas online con seña, cobros en el mostrador, caja por turno con arqueo, venta de '
      + 'productos con stock y reportes con Excel. Todo en el mismo panel, sin pasar datos de un lado a '
      + 'otro.',
  },
  {
    anim: 'anim-d3',
    question: '¿Mis clientes tienen que bajarse una app?',
    answer: 'No. Entran a tu link desde el celular y reservan. Sin app, sin registro, sin contraseña.',
  },
  {
    anim: 'anim-d4',
    question: '¿Cómo funciona la caja?',
    answer: 'Abrís la caja con el efectivo del cajón. Mientras está abierta, anotás ingresos y egresos, '
      + 'y los cobros en efectivo y las ventas entran solos. Al cerrar cargás lo que contaste y Vibe te '
      + 'muestra la diferencia: sobrante o faltante. Una caja abierta por vez, y la que se cierra no se '
      + 'reabre.',
  },
  {
    anim: 'anim-d5',
    question: '¿Qué pasa si vendo algo que figura sin stock?',
    answer: 'La venta sale igual. Vibe te avisa que ese producto quedó en cero o menos, para que revises '
      + 'el conteo. Y si le ponés un mínimo a cada producto, el dashboard te avisa cuando baja de ahí.',
  },
  {
    anim: 'anim-d6',
    question: '¿Qué pasa si un cliente cancela?',
    answer: 'Si cancela dentro del plazo que fijaste, MercadoPago le devuelve la seña solo. Vos no hacés '
      + 'nada.',
  },
  {
    anim: 'anim-d7',
    question: '¿Dónde cae la plata de las señas?',
    answer: 'Directo en tu cuenta de MercadoPago. Vibe nunca toca tu plata.',
  },
  {
    anim: 'anim-d8',
    question: '¿Y los que reservan y no vienen?',
    answer: 'La seña los filtra: el que pagó, viene. Y dos horas antes del turno, Vibe le manda un '
      + 'recordatorio automático al mail que dejó al reservar.',
  },
];
