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
import { CARGO_SERVICIO, CARGO_PUEDE_CAMBIAR, pesos } from './precio.ts';

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
    answer: 'Vibe no le cobra al complejo. Quien reserva online paga un cargo de servicio y lo ve antes de '
      + 'pagar, junto al precio de la cancha. La comisión de Mercado Pago es aparte.',
  },
  {
    anim: 'anim-d2',
    question: '¿Vibe es solo para reservas?',
    answer: 'No. Tiene reservas online con seña, turnos fijos semanales, canchas libres con inscripción, '
      + 'torneos con cuadro y resultados, caja por turno con arqueo, venta de productos con stock y '
      + 'reportes con Excel. Todo en el mismo panel.',
  },
  {
    anim: 'anim-d3',
    question: '¿Mis clientes tienen que bajarse una app?',
    answer: 'No. Entran a tu link, eligen cancha y horario y pagan la seña. No necesitan app ni cuenta.',
  },
  {
    anim: 'anim-d4',
    question: '¿Puedo cargar las reservas que me llegan por WhatsApp?',
    answer: 'Sí. Tocás el horario libre en la grilla y cargás la reserva con los datos del cliente. '
      + 'La anotás sin cobro, con seña o con el pago completo, y elegís cómo te pagó.',
  },
  {
    anim: 'anim-d5',
    question: '¿Puedo usar solo la caja y el bar?',
    answer: 'Sí. La caja por turno y la venta de productos funcionan aunque no recibas reservas online. '
      + 'Mercado Pago lo conectás cuando quieras.',
  },
  {
    anim: 'anim-d6',
    question: '¿Cómo funciona la caja?',
    answer: 'Abrís la caja con el efectivo del cajón. Mientras está abierta, anotás ingresos y egresos, '
      + 'y los cobros en efectivo y las ventas entran solos. Al cerrar cargás lo que contaste y Vibe te '
      + 'muestra la diferencia: sobrante o faltante. Una caja abierta por vez, y la que se cierra no se '
      + 'reabre.',
  },
  {
    anim: 'anim-d7',
    question: '¿Qué pasa si vendo algo que figura sin stock?',
    answer: 'La venta sale igual. Vibe te avisa que ese producto quedó en cero o menos, para que revises '
      + 'el conteo. Y si le ponés un mínimo a cada producto, el dashboard te avisa cuando baja de ahí.',
  },
  {
    anim: 'anim-d8',
    question: '¿Qué pasa si un cliente cancela?',
    answer: 'Si cancela dentro del plazo que fijaste, Mercado Pago le devuelve la seña solo. Vos no hacés '
      + 'nada.',
  },
  {
    anim: 'anim-d9',
    question: '¿Dónde cae la plata de las señas?',
    answer: 'Directo en tu cuenta de Mercado Pago. Vibe nunca toca tu plata.',
  },
  {
    anim: 'anim-d10',
    question: '¿Y los que reservan y no vienen?',
    answer: 'La seña filtra a quien no piensa ir. Y dos horas antes del turno, Vibe le manda un '
      + 'recordatorio automático al mail que dejó al reservar.',
  },
];
