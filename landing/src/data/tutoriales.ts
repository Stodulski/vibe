/**
 * Tutoriales = un video, una pantalla, una URL.
 *
 * No son guías. Una guía contesta algo que se busca ANTES de saber que Vibe
 * existe, y cita fuentes externas. Un tutorial muestra una pantalla del panel
 * funcionando, y su fuente es el producto. Van separados para que ninguna de
 * las dos cosas diluya a la otra: mezcladas, las guías dejan de leerse como
 * contenido editorial y los tutoriales dejan de encontrarse.
 *
 * `respuesta` va primero y por el mismo motivo que en las guías: un motor
 * generativo cita unidades que puede extraer enteras. El video no se puede
 * extraer, el párrafo sí, así que el párrafo tiene que decir lo mismo que el
 * video sin depender de él. Una página cuyo contenido está solo en el video es
 * una página vacía para todo el que no lo reproduce, incluido Google.
 *
 * `slug` es también el nombre del archivo en el CDN. Una sola cosa para
 * recordar, y el día que se regraba un video no hay que buscar a qué URL
 * correspondía.
 */
import type { Bloque } from './guias.ts';
import { fechaDeContenido, guardarFechas } from './fecha-de-contenido.ts';

export const CDN = 'https://cdn.vibe.com.ar/tutorials';

/** Alto y ancho reales de la grabación; van en el markup para que no salte el layout. */
export const VIDEO_ANCHO = 1920;
export const VIDEO_ALTO = 1080;

export type TutorialFaq = { question: string; answer: string };

export type Tutorial = {
  /** También el nombre del .mp4 y del .webp en el CDN. */
  slug: string;
  /** H1 de la página. */
  title: string;
  seoTitle: string;
  metaDescription: string;
  /** Lo que muestra el video, en un párrafo, sin depender de que se reproduzca. */
  respuesta: string;
  /** Bajada del índice. */
  excerpt: string;
  /** Segundos exactos de la grabación, para el VideoObject y el video-sitemap. */
  duracion: number;
  /**
   * El mismo video publicado en YouTube. Va en `sameAs` del VideoObject, no en
   * `contentUrl`: la página reproduce el archivo propio, y esto le dice a Google
   * que la copia de YouTube es el mismo contenido y no otro que compite.
   */
  youtube: string;
  datePublished: string;
  /* NO se escribe: sale del hash del contenido, igual que en las guías. */
  dateModified: string;
  bloques: Bloque[];
  faq: TutorialFaq[];
  /** Otros tutoriales que siguen naturalmente a este. */
  relacionados: string[];
  /** La guía que desarrolla el "qué hacer" de lo que este tutorial muestra. */
  guia?: { slug: string; texto: string };
};

const contenido: Omit<Tutorial, 'dateModified'>[] = [
  {
    slug: 'grilla-de-reservas',
    title: 'La grilla de reservas, día por día',
    seoTitle: 'Grilla de reservas de un complejo deportivo | Vibe',
    metaDescription:
      'Todas las canchas del día en una pantalla, el detalle de cada turno con su pago, y cómo '
      + 'bloquear un horario en el panel de Vibe.',
    excerpt: 'Todas tus canchas del día en una pantalla, y cada turno con su cliente y su pago.',
    duracion: 30,
    youtube: 'S7PUkxm8lu0',
    datePublished: '2026-09-16',
    respuesta:
      'La grilla muestra todas las canchas del complejo una al lado de la otra, con los turnos '
      + 'ocupados en su horario. Tocando cualquiera se abre el detalle: quién reservó, el horario, '
      + 'cuánto pagó de seña y cuánto queda por cobrar. Las flechas mueven la vista día por día, y '
      + 'el botón Hoy vuelve al día actual. Si una cancha queda fuera de servicio se bloquea el '
      + 'horario desde la misma pantalla, y ese turno deja de ofrecerse en la página pública de '
      + 'reservas.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Para qué sirve mirarla',
        parrafos: [
          'Es la pantalla que reemplaza al cuaderno. Cuando un cliente llama preguntando si hay lugar '
            + 'el jueves a las nueve, la respuesta está acá sin buscar en ningún lado: se ve qué está '
            + 'tomado y qué queda libre, en la cancha que el cliente quiere.',
          'También es donde se ve el estado de cobro sin abrir otra cosa. Un turno con la seña pagada y '
            + 'un turno reservado por teléfono se distinguen en el detalle, así que no hace falta '
            + 'cruzar la agenda con el registro de pagos para saber quién debe.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Qué se puede hacer desde la grilla',
        items: [
          'Ver el día completo con todas las canchas en paralelo',
          'Abrir un turno y ver el cliente, el horario y el estado del pago',
          'Moverte a otro día con las flechas, o volver a hoy',
          'Bloquear un horario para que no se pueda reservar online',
          'Cargar una reserva a mano, para el que reserva por teléfono',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué pasa cuando bloqueo un horario?',
        answer:
          'Ese horario desaparece de la página pública de reservas y nadie lo puede tomar. Se usa '
          + 'cuando una cancha está en mantenimiento, cuando hay un torneo o cuando la cancha se '
          + 'inundó. El bloqueo se saca de la misma forma.',
      },
      {
        question: '¿Puedo cargar una reserva que me pidieron por teléfono?',
        answer:
          'Sí. La reserva cargada a mano ocupa el turno igual que una reserva online, y queda en el '
          + 'historial del cliente. La diferencia es que no pasa por el cobro de la seña.',
      },
    ],
    relacionados: ['precios-por-horario', 'base-de-clientes'],
  },
  {
    slug: 'precios-por-horario',
    title: 'Precios por horario: la tarifa nocturna',
    seoTitle: 'Cómo poner precios por horario en una cancha | Vibe',
    metaDescription:
      'Cómo cargar un precio base por día y una franja nocturna más cara, y cómo se cobra un '
      + 'turno que cruza el límite entre las dos.',
    excerpt: 'Un precio base por día, y arriba las franjas que cobrás distinto.',
    duracion: 41,
    youtube: 'Zn91DlIh2rs',
    datePublished: '2026-09-16',
    respuesta:
      'Cada cancha arranca con un precio base para todo el día. Sobre ese precio se abre cualquier '
      + 'día de la semana y se le suma una franja con otro valor, que es como se carga la tarifa '
      + 'nocturna. El cobro no redondea a la franja donde empieza el turno: cada media hora se cobra '
      + 'a la tarifa que le corresponde a esa media hora. Un turno de una hora que arranca a las '
      + '17:30 con la franja nocturna empezando a las 18:00 se cobra media hora a precio de día y '
      + 'media hora a precio de noche.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Por qué el cálculo va por media hora',
        parrafos: [
          'Un complejo que cobra la noche más cara tiene siempre turnos que cruzan el límite. Si el '
            + 'sistema cobrara todo el turno a la tarifa del horario de inicio, reservar a las 17:30 '
            + 'saldría más barato que reservar a las 18:00 por la misma hora de cancha, y ese hueco lo '
            + 'encuentra el primer cliente que hace la cuenta.',
          'Cobrar cada media hora a su propia tarifa elimina el hueco sin que tengas que pensarlo. '
            + 'Cargás el precio de día y el de noche, y cualquier duración y cualquier horario de '
            + 'inicio dan el número correcto.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Qué más se edita desde la misma pantalla',
        items: [
          'El nombre de la cancha y el deporte que se juega',
          'Si es techada o descubierta',
          'La descripción que lee el cliente cuando elige dónde jugar',
          'Si la cancha está activa o deja de aparecer en reservas',
        ],
      },
    ],
    faq: [
      {
        question: '¿Puedo tener un precio distinto para cada día de la semana?',
        answer:
          'Sí. El precio base se carga por día, así que el sábado puede valer distinto que el martes '
          + 'sin tocar los demás días. Y sobre cada día se pueden sumar franjas con su propio valor.',
      },
      {
        question: '¿Cuántas franjas puedo cargar por día?',
        answer:
          'Las que necesites. Lo habitual es una sola, la nocturna, pero un complejo que cobra '
          + 'distinto al mediodía puede cargar esa también. Lo que no se puede es superponer dos '
          + 'franjas en el mismo horario.',
      },
    ],
    relacionados: ['grilla-de-reservas', 'reportes-de-facturacion'],
    guia: {
      slug: 'cuanto-cuesta-un-sistema-de-reservas-para-canchas',
      texto: 'Cuánto cuesta un sistema de reservas para canchas',
    },
  },
  {
    slug: 'base-de-clientes',
    title: 'La base de clientes se arma sola',
    seoTitle: 'Base de clientes de un complejo deportivo | Vibe',
    metaDescription:
      'Cada persona que reserva queda guardada con su teléfono, su historial y su porcentaje de '
      + 'asistencia, sin cargar una ficha a mano.',
    excerpt: 'Cada persona que reserva queda guardada, con su historial y su asistencia.',
    duracion: 38,
    youtube: 'c5QQGWhU3Es',
    datePublished: '2026-09-16',
    respuesta:
      'La base de clientes no se carga: se llena sola con cada reserva. De cada persona queda el '
      + 'teléfono, cuántas veces reservó, cuántas veces faltó y un porcentaje de asistencia. El '
      + 'buscador encuentra por nombre o por parte del teléfono, y la ficha de cada cliente muestra '
      + 'sus últimas reservas con la cancha, la fecha y si se presentó.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Para qué sirve el porcentaje de asistencia',
        parrafos: [
          'Es el dato que decide cuándo pedir seña y cuándo no. Un cliente que viene hace dos años y '
            + 'no faltó nunca no necesita el mismo trato que alguien que reservó tres veces y se '
            + 'presentó una. Sin el dato, esa decisión se toma de memoria y la memoria falla justo con '
            + 'el que menos conviene.',
          'También sirve para el caso inverso: saber a quién avisarle primero cuando se libera un '
            + 'horario bueno o cuando querés llenar un martes flojo.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Cómo se marca que un cliente no se presentó?',
        answer:
          'Desde el turno en la grilla. Esa marca es la que alimenta el porcentaje de asistencia, así '
          + 'que el dato vale tanto como la constancia con que se carga.',
      },
      {
        question: '¿Tengo que pedirle el email al cliente?',
        answer:
          'No es obligatorio. Con el nombre y el teléfono alcanza para reservar. El email, si lo deja, '
          + 'se usa para mandarle la confirmación y el recordatorio del turno.',
      },
    ],
    relacionados: ['grilla-de-reservas', 'panel-de-control'],
  },
  {
    slug: 'panel-de-control',
    title: 'El panel de control del complejo',
    seoTitle: 'Panel de control de un complejo deportivo | Vibe',
    metaDescription:
      'Facturación del día, reservas, ocupación y el mapa de horarios muertos: qué muestra el '
      + 'panel de Vibe apenas entrás.',
    excerpt: 'Cuánto facturaste, cuántas reservas tenés y qué canchas están vacías, al entrar.',
    duracion: 33,
    youtube: 'EfSBAkqQHCk',
    datePublished: '2026-09-16',
    respuesta:
      'Al entrar al panel se ve la facturación del día, la cantidad de reservas y el porcentaje de '
      + 'ocupación de las canchas. Arriba queda el enlace público de reservas, listo para copiar. '
      + 'Más abajo están el gráfico de ingresos, que se mira por semana o por mes, los clientes que '
      + 'más reservaron, y el mapa de ocupación por horario, que es donde se ven las horas que '
      + 'quedan vacías.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'El mapa de ocupación es el que más se mira',
        parrafos: [
          'La facturación del día dice cómo te fue. El mapa de ocupación dice dónde está lo que '
            + 'todavía no cobraste: qué horarios quedan vacíos semana tras semana, que es distinto de '
            + 'un día flojo suelto.',
          'Un complejo lleno los viernes a la noche y vacío los martes a las tres de la tarde no tiene '
            + 'un problema de demanda, tiene un problema de franja. El mapa lo muestra sin que haya '
            + 'que llevar la cuenta a mano.',
        ],
      },
    ],
    faq: [
      {
        question: '¿El enlace de reservas es el mismo siempre?',
        answer:
          'Sí, no cambia. Es el que conviene dejar fijo en la biografía de Instagram y en el estado de '
          + 'WhatsApp, porque una vez que está publicado en varios lados no querés que se rompa.',
      },
      {
        question: '¿Puedo ver los ingresos de un mes anterior?',
        answer:
          'El gráfico del panel muestra la tendencia reciente. Para el detalle de un mes cerrado, con '
          + 'el desglose por método de pago y por cancha, está la pantalla de reportes.',
      },
    ],
    relacionados: ['reportes-de-facturacion', 'base-de-clientes'],
    guia: {
      slug: 'como-llenar-los-horarios-vacios-de-un-complejo',
      texto: 'Cómo llenar los horarios vacíos de un complejo',
    },
  },
  {
    slug: 'reportes-de-facturacion',
    title: 'Reportes de facturación y exportación',
    seoTitle: 'Reporte de facturación de un complejo | Vibe',
    metaDescription:
      'El cierre del mes comparado con el mes anterior, el desglose por método de pago y por '
      + 'cancha, y la exportación a Excel para el contador.',
    excerpt: 'El cierre del mes armado solo, comparado con el anterior y listo para el contador.',
    duracion: 27,
    youtube: 'n4h7--y2sd4',
    datePublished: '2026-09-16',
    respuesta:
      'El reporte se arma mes a mes y se compara siempre contra el mes anterior. Muestra cuánto '
      + 'entró en efectivo, cuánto por Mercado Pago y cuánto por transferencia, con los reembolsos '
      + 'ya descontados, así que el número que se ve es el neto. También se abre cancha por cancha, '
      + 'para ver cuáles facturan y cuáles casi no se usan. Todo eso se descarga en Excel.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Por qué importa el desglose por cancha',
        parrafos: [
          'El total del mes no dice qué hacer. El desglose por cancha sí: una cancha que factura la '
            + 'mitad que las demás está indicando un precio mal puesto, un horario mal cargado o algo '
            + 'que arreglar en la cancha misma.',
          'Cruzado con el mapa de ocupación del panel, separa los dos casos que se confunden: la '
            + 'cancha que se usa poco y la cancha que se usa mucho pero barata.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué formato tiene la exportación?',
        answer:
          'Un archivo de Excel con los pagos del período. Se abre igual en Google Sheets si no usás '
          + 'Excel, y es el formato que suele pedir el contador.',
      },
      {
        question: '¿Los reembolsos están descontados?',
        answer:
          'Sí. El neto que muestra el reporte ya tiene los reembolsos restados, así que no hay que '
          + 'hacer la cuenta aparte.',
      },
    ],
    relacionados: ['panel-de-control', 'precios-por-horario'],
    guia: {
      slug: 'cuanto-cobra-mercadopago-por-una-sena',
      texto: 'Cuánto cobra MercadoPago por cobrar una seña',
    },
  },
  {
    slug: 'configuracion-del-complejo',
    title: 'Configurar el complejo: horarios y seña',
    seoTitle: 'Configurar horarios y seña de un complejo | Vibe',
    metaDescription:
      'Los datos del complejo, hasta qué hora se puede reservar cada día, el porcentaje de seña '
      + 'y el estado de la conexión con Mercado Pago.',
    excerpt: 'Los datos del complejo, los horarios de atención y cuánta seña pedís.',
    duracion: 25,
    youtube: '05OH3m9ZJxM',
    datePublished: '2026-09-16',
    respuesta:
      'En Configuración se cargan el nombre, la dirección y el teléfono del complejo, y el '
      + 'porcentaje de seña que se le pide al cliente al reservar online. Los horarios de atención '
      + 'se definen día por día, y fuera de esa franja la página pública no ofrece turnos. En la '
      + 'pestaña de cobros se ve si Mercado Pago está conectado y cuánto cobra según el plazo de '
      + 'acreditación elegido.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'La seña es la decisión que más cambia el resultado',
        parrafos: [
          'El porcentaje de seña es lo único de esta pantalla que se elige y no se copia de un dato '
            + 'que ya existe. Una seña baja hace que reservar sea fácil y que faltar también lo sea. '
            + 'Una seña alta filtra al que no va a ir, y también a alguno que sí iba.',
          'Es un número para revisar contra el porcentaje de ausencias que muestra la base de clientes, '
            + 'no para dejarlo donde quedó el primer día.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Qué pasa si no conecto Mercado Pago?',
        answer:
          'El complejo funciona igual y las reservas se pueden cargar, pero no se cobra la seña '
          + 'online. Conectarlo es lo que permite que el cliente pague al reservar y que la plata '
          + 'entre directo a tu cuenta.',
      },
      {
        question: '¿Puedo tener horarios distintos los fines de semana?',
        answer:
          'Sí, los horarios de atención se definen día por día. El sábado puede abrir y cerrar a otra '
          + 'hora que el lunes, y la página pública respeta cada uno.',
      },
    ],
    relacionados: ['precios-por-horario', 'varios-complejos'],
    guia: {
      slug: 'cuanto-cobra-mercadopago-por-una-sena',
      texto: 'Cuánto cobra MercadoPago por cobrar una seña',
    },
  },
  {
    slug: 'varios-complejos',
    title: 'Varios complejos con una sola cuenta',
    seoTitle: 'Administrar varias sedes con una cuenta | Vibe',
    metaDescription:
      'Varias sedes desde la misma cuenta: cada una con sus canchas, sus precios y su propio '
      + 'enlace de reservas, sin mezclarse.',
    excerpt: 'Cuatro sedes, una cuenta, y cada una con sus canchas y sus números aparte.',
    duracion: 26,
    youtube: 'WlB2BoiVy30',
    datePublished: '2026-09-16',
    respuesta:
      'Una misma cuenta puede administrar varias sedes. Se cambia de una a otra desde el selector de '
      + 'arriba, sin cerrar sesión. Cada sede mantiene sus propias canchas, sus precios, sus horarios '
      + 'y su propio enlace público de reservas, y los datos no se mezclan entre ellas: la '
      + 'facturación, los clientes y la ocupación son de la sede en la que estás parado.',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Por qué los datos van separados',
        parrafos: [
          'Dos sedes en ciudades distintas tienen precios distintos, horarios distintos y clientes '
            + 'distintos. Un total que las sume oculta exactamente lo que hay que mirar, que es cuál de '
            + 'las dos está funcionando.',
          'Cada sede tiene además su propio enlace de reservas, así que el cliente que entra por la '
            + 'sede de su barrio ve las canchas de esa sede y no las de la otra punta de la provincia.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Puedo tener un empleado que vea solo una sede?',
        answer:
          'El selector muestra las sedes de la cuenta con la que se entró. Para separar accesos por '
          + 'sede, lo que corresponde es una cuenta por sede, con su propio usuario.',
      },
      {
        question: '¿Cuántas sedes puedo cargar?',
        answer:
          'No hay un tope. El costo de Vibe es por reserva cobrada, no por cantidad de sedes ni de '
          + 'canchas, así que sumar una sede no cambia lo que pagás por tenerla cargada.',
      },
    ],
    relacionados: ['panel-de-control', 'configuracion-del-complejo'],
  },
];

/* La fecha sale del contenido, igual que en las guías. Las claves se prefijan
   para que un tutorial y una guía con el mismo slug nunca se pisen en el
   registro compartido. */
export const tutoriales: Tutorial[] = contenido.map(t => {
  const { datePublished, ...loQueSeMide } = t;
  return { ...t, dateModified: fechaDeContenido(`tutorial:${t.slug}`, loQueSeMide) };
});
guardarFechas();

export const getTutorial = (slug: string) => tutoriales.find(t => t.slug === slug);

export const getRelacionados = (t: Tutorial) =>
  t.relacionados.map(getTutorial).filter((x): x is Tutorial => Boolean(x));

/** Fecha del tutorial más nuevo, para el índice. */
export const ULTIMA_ACTUALIZACION = tutoriales
  .map(t => t.dateModified)
  .sort()
  .at(-1) ?? '2026-09-16';
