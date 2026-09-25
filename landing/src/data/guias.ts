/**
 * Guías = una pregunta, una URL.
 *
 * No son un blog. Cada guía existe porque es algo que un dueño de complejo
 * busca literalmente en Google antes de saber que Vibe existe, y la home no
 * puede rankear para todas: una página compite por una intención.
 *
 * La forma es siempre la misma y no es decorativa. Primero `respuesta`, que
 * contesta la pregunta completa en un párrafo, con el número adelante. Recién
 * después el desarrollo. Un motor generativo cita unidades que puede extraer
 * enteras, y un párrafo que arranca con "depende de varios factores" no es una.
 *
 * Las preguntas de `faq` son más específicas que las de la home a propósito:
 * repetir la misma pregunta en dos FAQPage duplica el schema y compite contra
 * sí mismo.
 */
/* La extension va explicita porque tests/smoke.mjs importa este archivo con el
   stripping de TypeScript de Node, que a diferencia de Vite no resuelve rutas
   relativas sin extension. `allowImportingTsExtensions` ya esta activo en el
   tsconfig de Astro, asi que del lado del build no cambia nada. */
import {
  COSTOS, PLAZOS, VIGENTE_DESDE, FUENTE, MINIMO, MAXIMO, porcentaje,
  tasaDe, fechaEnTexto, BRECHA_INMEDIATA, IVA, RECARGO_TARJETA_EXTRANJERA,
  RANGO_NACIONAL,
} from './mercadopago-costos.ts';
import { ejemplo, pesos, dolares } from './precio.ts';
import { fechaDeContenido, guardarFechas } from './fecha-de-contenido.ts';
import {
  ATC_PLANES, ATC_FUENTE, ATC_VERIFICADO, ATC_PRUEBA_GRATIS_DIAS,
  ATC_DESCUENTO_ANUAL_PORCENTAJE,
  ATC_DESCUENTO_SEGUN_TABLA_PORCENTAJE,
  CF_PLANES, CF_FUENTE, CF_VERIFICADO, CF_FEE_AL_JUGADOR, CF_SOPORTE_HORARIO,
  CF_MINUTOS_DE_CONFIGURACION, CF_REEMBOLSO_DIAS_HABILES,
  TU_PLANES, TU_FUENTE, TU_VERIFICADO,
} from './competencia.ts';
import {
  filasDeCostoPorReserva, columnasDeCostoPorReserva, cargoPorReserva,
  POR_RESERVA_POCAS, POR_RESERVA_MUCHAS, VECES, VOLUMENES,
} from './costo-por-reserva.ts';
import { CARGO_MINIMO, CARGO_SERVICIO_TEXTO, SENA_DE_EJEMPLO } from './precio.ts';

/* Los extremos de la banda, buscados por provincia en vez de tipeados. */
const BA_INMEDIATA = tasaDe('Buenos Aires', 'Al instante');
const BA_LENTA = tasaDe('Buenos Aires', '35 días');
const CHACO_LENTA = tasaDe('Chaco', '35 días');
const e = ejemplo();

export type Bloque =
  | { tipo: 'parrafos'; heading: string; parrafos: string[] }
  | { tipo: 'lista'; heading: string; intro?: string; items: string[] }
  | {
      tipo: 'tabla';
      heading: string;
      intro?: string;
      nota?: string;
      columnas: string[];
      filas: string[][];
    };

export type GuiaFaq = { question: string; answer: string };

export type Guia = {
  slug: string;
  /** H1 de la página. */
  title: string;
  seoTitle: string;
  metaDescription: string;
  /** La respuesta completa, en un párrafo, antes de cualquier desarrollo. */
  respuesta: string;
  /** Bajada del índice. */
  excerpt: string;
  /* Se escribe una vez, cuando la guia sale. No deriva y no se mueve. */
  datePublished: string;
  /* NO se escribe: sale del hash del contenido (ver fecha-de-contenido.ts).
     Estaba tipeada, y era el mismo bug que perseguimos en todo lo demas con el
     agravante de que ademas viaja en el JSON-LD, donde la frescura se mira. */
  dateModified: string;
  /** De dónde salen los datos, si la guía afirma números que no son propios. */
  fuente?: { nombre: string; url: string; nofollow?: boolean };
  /**
   * El tutorial que muestra en el panel lo que esta guía explica.
   *
   * Es el recíproco del campo `guia` de un tutorial, y va escrito a mano en vez
   * de importado: `tutoriales.ts` ya importa el tipo `Bloque` de acá, así que
   * importar de vuelta cerraría el ciclo. El día que un slug cambie, el chequeo
   * de huérfanas del seo-gate lo caza.
   *
   * Importa que exista: las guías son las únicas páginas con impresiones
   * propias, así que son la puerta de entrada. Sin esta salida, el que llega
   * buscando una respuesta nunca ve el producto funcionando.
   */
  tutorial?: { slug: string; texto: string };
  /**
   * Marca las guías que comparan Vibe con un competidor concreto.
   *
   * La página las usa para enlazarlas entre sí sin que nadie mantenga la lista:
   * quien está evaluando una alternativa casi siempre está mirando dos o tres a
   * la vez, y hasta ahora tenía que volver al índice para pasar de una a otra.
   * Una comparación nueva se suma sola.
   */
  comparacion?: boolean;
  /** Para la guía que habla del tema y conviene que ofrezca las comparaciones. */
  verComparaciones?: boolean;
  bloques: Bloque[];
  faq: GuiaFaq[];
};

/* La tabla de costos se arma desde el dataset y no se transcribe a mano: es el
   mismo dato que usa la sección de precio de la home, y dos copias del mismo
   número terminan siempre con una desactualizada. */
const filasDeCostos = COSTOS.map(g => [
  g.provincias.join(', '),
  ...g.tasas.map(porcentaje),
]);

const contenido: Omit<Guia, 'dateModified'>[] = [
  {
    slug: 'vibe-o-turnito-para-un-complejo-de-canchas',
    comparacion: true,
    tutorial: { slug: 'grilla-de-reservas', texto: 'Cómo se ve la grilla de reservas de Vibe' },
    title: 'Vibe o Turnito: una agenda o un sistema para tus canchas',
    seoTitle: 'Vibe o Turnito para un complejo de canchas: qué cambia',
    metaDescription:
      'Turnito es una agenda de turnos; Vibe, un sistema para canchas. Qué cambia con varias canchas '
      + 'y quién paga la comisión en cada uno.',
    excerpt:
      'Una agenda no es una grilla. Con varias canchas lo sentís todos los días, y también en quién '
      + 'paga la comisión.',
    respuesta:
      'Turnito es una agenda de turnos para peluquerías, consultorios, gimnasios y clubes; Vibe es un '
      + 'sistema de gestión para complejos de canchas. La diferencia salta en la pantalla de reserva: '
      + 'en Turnito, el que reserva elige una fecha y ve una lista plana de horarios, sin nada que '
      + 'nombre una cancha; en Vibe ve todas las canchas del día en paralelo. En plata, Turnito tiene '
      + `un plan gratis y planes pagos desde ${pesos(TU_PLANES[1].mensual)} por mes, y en todos menos el `
      + `más caro cobra una comisión sobre lo que se paga online, que baja de ${TU_PLANES[0].comision} `
      + 'por ciento a cero según el plan y absorbe el complejo. Vibe no le cobra nada al complejo: el '
      + 'cargo de servicio lo paga el que reserva.',
    datePublished: '2026-09-16',
    fuente: { nombre: 'Turnito, planes de Argentina', url: TU_FUENTE, nofollow: true },
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Con cuatro canchas, una agenda se queda corta',
        parrafos: [
          'Con una cancha, una agenda alcanza. Con cuatro, cada cancha es una agenda aparte: el que '
            + 'reserva entra a una, mira sus horarios y, si no hay lugar, sale y prueba otra. No ve el '
            + 'día completo. No elige: adivina.',
          'Del lado tuyo pasa lo mismo. Para saber cómo viene el sábado abrís una agenda por cancha, en '
            + 'vez de mirar una sola pantalla. Una lista contra una grilla, todos los días.',
          'Y hay un tope que conviene mirar antes que el precio: el plan gratuito permite tres agendas '
            + 'y el segundo, cinco. Si cada cancha es una agenda, un complejo de seis canchas arranca '
            + `recién en el tercer plan, que sale ${pesos(TU_PLANES[2].mensual)} por mes.`,
        ],
      },
      {
        tipo: 'lista',
        heading: 'Lo que Turnito hace bien',
        intro: 'Es cierto, y a un complejo le sirve:',
        items: [
          'Un plan gratis que no vence, con cien reservas por mes',
          'Turnos recurrentes flexibles: diarios, semanales o mensuales, con fecha de fin o sin ella',
          'Calendario embebido, para que la reserva viva adentro de tu propia web',
          'Bloqueo automático del cliente que acumula faltas',
          'Recordatorios por WhatsApp, Telegram y mail',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Lo que Vibe trae para un complejo',
        intro: 'Vibe está hecho para canchas, y se nota en todo lo que pasa alrededor de la reserva:',
        items: [
          'Todas las canchas del día en una grilla, para vos y para el que reserva',
          'Seña cobrada con MercadoPago al reservar, y reembolso automático si cancela a tiempo',
          'Cobros en el mostrador: efectivo, transferencia, débito, crédito o QR',
          'Caja por turno con arqueo, y venta de productos con stock',
          'Sin planes por cantidad de canchas: sumar una no te cambia nada',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Para quién es cada uno',
        parrafos: [
          'Turnito es una agenda genérica: además de canchas sirve para peluquerías, consultorios y '
            + 'gimnasios. Si en tu complejo también das clases con profes que llevan agenda propia, o '
            + 'alquilás otros espacios que no son cancha, una herramienta genérica te puede cubrir todo '
            + 'eso con un solo sistema.',
          'Si no querés que tu cliente vea ningún cargo aparte del precio de la cancha, Turnito va al '
            + 'revés que Vibe: la comisión la absorbe el complejo y el que reserva ve un total sin '
            + 'desglose.',
          'Vibe está hecho para complejos que alquilan canchas por hora: el que reserva ve todas las '
            + 'canchas del día en una sola grilla, sin un plan que limite cuántas cargás. El cargo de '
            + 'servicio lo paga el que reserva, y la caja, los cobros del mostrador y el bar están en el '
            + 'mismo sistema. Usar Vibe es gratis para el complejo, tengas una cancha o veinte.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Turnito sirve para un complejo de pádel?',
        answer:
          'Para tomar reservas, sí. Lo que no tiene es una vista con todas las canchas del día juntas: '
          + 'cada cancha es una agenda separada, y el que reserva entra a una por vez.',
      },
      {
        question: '¿Qué pasa si me paso de las cien reservas del plan gratis?',
        answer:
          'No está publicado. Ni la página de planes ni los términos dicen si se bloquea, si se cobra '
          + 'un excedente o si se corta. Preguntalo antes de apoyarte en ese plan.',
      },
      {
        question: '¿La comisión la paga el complejo o el cliente?',
        answer:
          'En Turnito, el complejo: la plata va directo a la cuenta del negocio y el que reserva ve un '
          + 'total sin línea de comisión. Aparte está la de MercadoPago, que cobra siempre a quien '
          + 'recibe el pago, con cualquier plataforma. En Vibe es al revés: la plataforma no le cobra '
          + 'nada al complejo, el cargo de servicio lo paga el que reserva, y lo único que pagás vos es '
          + 'la comisión de MercadoPago sobre el pago online.',
      },
      {
        question: '¿Esta comparación está actualizada?',
        answer:
          `Los planes se leyeron de su página el ${TU_VERIFICADO} y el link está arriba. Turnito cambia `
          + 'los precios por país: estos son los de Argentina.',
      },
      {
        question: 'Si sumo canchas, ¿tengo que cambiar de plan en Vibe?',
        answer:
          'No. Vibe no cobra por plan ni por abono, así que sumar una cancha no te mueve a nada más '
          + 'caro. En Turnito cada cancha ocupa una agenda, y el plan depende de cuántas sumes.',
      },
    ],
  },
  {
    slug: 'vibe-o-canchafija-en-que-se-diferencian',
    comparacion: true,
    tutorial: { slug: 'panel-de-control', texto: 'El panel de control de Vibe, por dentro' },
    title: 'Vibe o CanchaFija: qué comparten y en qué se diferencian',
    seoTitle: 'Vibe o CanchaFija: en qué se diferencian y cuánto cuestan',
    metaDescription:
      'CanchaFija suma turnos fijos, torneos, escuelitas y socios, con abono mensual. Vibe cubre '
      + 'reservas, caja y bar sin cobrarle al complejo.',
    excerpt:
      'Comparten la reserva, el cobro y el bar. Uno suma la vida de club y cobra abono; el otro no le '
      + 'cobra al complejo.',
    respuesta:
      'CanchaFija y Vibe comparten la base: reservas online con cobro por MercadoPago y la gestión del '
      + 'bar o la cantina. CanchaFija suma lo que hace a la vida de un club: turnos fijos semanales, '
      + 'torneos, escuelitas con cuotas y socios con cobro automático. La diferencia de plata es el '
      + `abono: CanchaFija le cobra al complejo un plan mensual en pesos según cuántas canchas tenga, `
      + `desde ${pesos(CF_PLANES[0].mensual)} hasta ${pesos(CF_PLANES.at(-1)!.mensual)}, con el primer `
      + 'mes gratis y sin permanencia. Vibe no le cobra nada al complejo, e incluye caja por turno con '
      + 'arqueo y venta de productos con stock.',
    datePublished: '2026-09-16',
    fuente: { nombre: 'CanchaFija, planes y precios', url: CF_FUENTE, nofollow: true },
    bloques: [
      {
        tipo: 'lista',
        heading: 'Lo que hacen las dos',
        intro: 'Arranquemos por lo que tienen en común, que es bastante:',
        items: [
          'Reserva online con cobro por MercadoPago',
          'El bar o la cantina: en Vibe, venta de productos con stock y caja por turno con arqueo',
          'Reportes de lo que entra',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Lo que CanchaFija suma para la vida de club',
        intro: 'CanchaFija va más allá de la cancha con estas herramientas:',
        items: [
          'Turnos fijos: la reserva recurrente semanal, con la opción de saltear una semana sin romper la serie',
          'Torneos, con fixture, inscripciones y premios',
          'Escuelitas con cuotas mensuales que se generan solas',
          'Socios, con cobro automático y control de asistencia',
          'Tienda online, incluida en todos los planes',
          'Canchas combinables, para partir una de fútbol 7 en dos de fútbol 5',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Dos formas de cobrarle al que reserva',
        parrafos: [
          'Las dos le cobran al jugador, no al complejo, pero lo muestran distinto. CanchaFija lo dice '
            + `así en sus términos: "${CF_FEE_AL_JUGADOR}". El cargo existe y va adentro del número que `
            + 've el jugador, sin desglosar.',
          'Vibe lo muestra aparte: el cliente ve el precio de la cancha y, abajo, el cargo de servicio. '
            + 'Ninguna forma es la correcta. Adentro, la compra es más simple y no se ve de dónde sale; '
            + 'aparte, es más transparente y deja a la vista un número que alguien puede discutir.',
          'Un dato que falta: CanchaFija no publica el porcentaje de su fee, ni en precios ni en '
            + 'términos. No hay forma de calcular de antemano cuánto le cuesta la reserva a tu cliente.',
          'Y hay una comisión que no es de ninguna de las dos: la de MercadoPago, que se queda con una '
            + 'parte de cada pago online, a cargo de quien lo recibe, con cualquier plataforma. Con '
            + 'CanchaFija pagás esa comisión más el abono; con Vibe, esa comisión y nada más (mirá '
            + '<a href="/guias/cuanto-cobra-mercadopago-por-una-sena">cuánto cobra MercadoPago por una '
            + 'seña</a>).',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Cómo elegir',
        parrafos: [
          'Si tu complejo funciona como un club —con socios, escuelitas y torneos—, CanchaFija está '
            + 'armado para esa operación completa.',
          'Un detalle operativo: el alta en CanchaFija no es automática. Completás un formulario y '
            + `ellos te mandan las credenciales; dicen que configurarlo lleva unos `
            + `${CF_MINUTOS_DE_CONFIGURACION} minutos una vez adentro, y el soporte es el mismo en `
            + `todos los planes, de ${CF_SOPORTE_HORARIO}. En Vibe la cuenta la creás vos, sin esperar `
            + 'a nadie.',
          'Si lo tuyo es alquilar canchas por hora, cobrar la seña y llevar la caja y el bar, las dos '
            + 'lo resuelven y la diferencia es el abono. CanchaFija cobra su plan todos los meses, haya '
            + 'reservas o no. Vibe no le cobra nada a tu complejo. En un enero flojo, eso se siente.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Cuánto le cobra CanchaFija al jugador por reserva?',
        answer:
          'No está publicado. Sus términos dicen que el fee va incluido en el precio, pero no de cuánto '
          + 'es, y sus páginas públicas no muestran el precio de un turno.',
      },
      {
        question: '¿Cómo funcionan las cancelaciones en CanchaFija?',
        answer:
          'La política la fija cada complejo, no la plataforma. Fuera del plazo que define, o si el '
          + `jugador no se presenta, no hay reembolso; cuando corresponde, puede tardar hasta `
          + `${CF_REEMBOLSO_DIAS_HABILES} días hábiles.`,
      },
      {
        question: '¿Esta comparación está actualizada?',
        answer:
          `Los planes se leyeron de su página el ${CF_VERIFICADO} y el link está arriba. Son precios en `
          + 'pesos: confirmalos antes de hacer cuentas finas.',
      },
      {
        question: '¿Vibe me cobra algo si un mes no tengo reservas?',
        answer:
          'No. Vibe no cobra abono ni mínimo, y tampoco cobra sobre la caja o el bar: un mes sin '
          + 'reservas es un mes sin ningún cargo de Vibe. CanchaFija cobra su plan haya reservas o no.',
      },
    ],
  },
  {
    slug: 'vibe-o-atc-sports-en-que-se-diferencian',
    comparacion: true,
    tutorial: { slug: 'grilla-de-reservas', texto: 'Cómo se ve la grilla de reservas de Vibe' },
    title: 'Vibe o ATC Sports: en qué se diferencian de verdad',
    seoTitle: 'Vibe o ATC Sports: abono fijo o $0 para tu complejo',
    metaDescription:
      'ATC Sports cobra un abono mensual en dólares; Vibe no le cobra nada al complejo. Qué comparten, '
      + 'qué trae cada uno y cuándo conviene.',
    excerpt:
      'Las dos toman reservas y llevan la caja. La diferencia está en quién paga, cuándo, y en lo que '
      + 'ATC suma afuera de la cancha.',
    respuesta:
      'La diferencia principal no está en las reservas, que las dos cubren: está en quién paga. ATC '
      + `Sports le cobra al complejo un abono mensual fijo en dólares, desde ${dolares(ATC_PLANES[0].mensual)} `
      + `hasta ${dolares(ATC_PLANES.at(-1)!.mensual)} según cuántas canchas tengas, haya reservas o no. `
      + 'Vibe no le cobra nada al complejo: el cargo de servicio lo paga el cliente sobre la seña, y la '
      + 'comisión de MercadoPago corre con los dos modelos. En alcance, las dos cubren reservas, caja y '
      + 'stock, y cada complejo tiene su propia página pública para que el cliente vea las canchas y '
      + 'reserve. ATC suma funciones extra —grabación de partidos, banners QR y accesos para varios '
      + 'usuarios del staff— que se detallan más abajo.',
    datePublished: '2026-09-16',
    fuente: { nombre: 'ATC Sports, software de gestión deportiva', url: ATC_FUENTE, nofollow: true },
    bloques: [
      {
        tipo: 'lista',
        heading: 'Lo que hacen las dos',
        intro:
          'Empecemos por acá, porque es la mayor parte. En estas cosas, elegir una u otra no cambia lo '
          + 'que vas a poder hacer:',
        items: [
          'Reserva online, sin contestar un solo mensaje',
          'Grilla de turnos con todas las canchas del día',
          'Datos de cada cancha y de cada cliente',
          'Caja y control de stock para lo que vendés en el mostrador',
          'Reportes de lo que facturaste',
          'Acceso desde el celular',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'La diferencia real: quién paga y cuándo',
        parrafos: [
          'ATC cobra un abono mensual por complejo, en dólares, escalonado por cantidad de canchas. Es '
            + 'previsible: sabés lo que pagás el mes que viene, con un enero flojo o un agosto lleno. '
            + `Ofrece ${ATC_PRUEBA_GRATIS_DIAS} días de prueba gratis y un ${ATC_DESCUENTO_ANUAL_PORCENTAJE} `
            + 'por ciento de descuento si pagás el año por adelantado. Ese es el número que anuncian; '
            + `los precios de su propia tabla dan un ${ATC_DESCUENTO_SEGUN_TABLA_PORCENTAJE} por ciento.`,
          'Vibe no cobra abono. El complejo no le paga nada a la plataforma; lo que hay es un cargo de '
            + 'servicio sobre la seña, y lo paga el que reserva. Un mes sin reservas no te cuesta un '
            + 'peso de Vibe. La contracara: tu cliente ve un importe un poco más alto al reservar.',
          'Hay una comisión que corre con los dos: la de MercadoPago, que se descuenta del pago online '
            + 'a quien lo recibe, con cualquier plataforma. Con ATC se suma al abono; con Vibe es lo '
            + 'único que paga el complejo (mirá '
            + '<a href="/guias/cuanto-cobra-mercadopago-por-una-sena">cuánto cobra MercadoPago por una '
            + 'seña</a>).',
          'Ninguno de los dos modelos es mejor en abstracto. Con Vibe, el complejo le paga cero a la '
            + 'plataforma, tenga cinco reservas en el mes o quinientas: lo que crece con el volumen no '
            + 'es tu costo, es lo que pagan tus clientes en total. Con un abono es al revés: el costo es '
            + 'tuyo y es fijo, y tus clientes no pagan nada extra sobre la cancha.',
          'Así que la pregunta no es cuál sale más barato, sino quién querés que cargue con el costo. Si '
            + 'te importa que tu cliente vea exactamente el precio de la cancha y nada más, ese es un '
            + 'motivo real para elegir un abono, y pesa más cuanto más volumen tenés.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Funciones extra que suma ATC',
        intro: `Publicado en su página, verificado el ${ATC_VERIFICADO}, más allá de reservas y caja:`,
        items: [
          'Integración con grabación de partidos',
          'Banners QR y paquetes digitales personalizados',
          'Sitio web propio del complejo, más allá de la página de reservas',
          'Multiusuario: varias personas con su propio acceso al mismo complejo',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Cómo elegir sin probar los dos',
        parrafos: [
          'Si querés grabar los partidos o que varias personas del staff entren con su propio usuario, '
            + 'eso lo tiene ATC. La reserva online, la caja y el stock del bar los tienen las dos por '
            + 'igual.',
          'Si lo que necesitás es que la gente reserve y pague la seña sola, la decisión se reduce a '
            + 'quién carga con el costo: vos con una cuota fija, o tu cliente con un cargo sobre la '
            + 'seña. Cuánto sale un abono por turno según tu volumen está calculado, con la tabla '
            + 'completa, en <a href="/guias/cuanto-cuesta-un-sistema-de-reservas-para-canchas">cuánto '
            + 'cuesta un sistema de reservas</a>.',
          'Y si estás arrancando o tu temporada baja es muy baja, el mejor argumento para no tener '
            + 'abono no es el precio: es que no tenés que acertarle a la demanda. Te podés equivocar '
            + 'sin que te cueste plata todos los meses.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Puedo pasarme de ATC a Vibe sin perder las reservas?',
        answer:
          'Las reservas futuras se cargan a mano desde la grilla, como una reserva telefónica. El '
          + 'historial viejo no se importa: la base de clientes de Vibe se arma sola con las reservas '
          + 'nuevas.',
      },
      {
        question: '¿Vibe tiene prueba gratis?',
        answer:
          'No la necesita: como no hay abono, no hay nada que probar antes de pagar. Cargás el complejo '
          + 'y lo usás. El cargo de servicio aparece recién cuando alguien reserva online, y lo paga él.',
      },
      {
        question: '¿Puedo absorber yo el cargo de servicio?',
        answer:
          'El cargo se le suma al cliente al pagar la seña. Si no querés que lo sienta, podés bajar el '
          + 'precio de la cancha para compensarlo: es una decisión de precio tuya.',
      },
      {
        question: '¿Esta comparación está actualizada?',
        answer:
          `Los datos de ATC se leyeron de su página el ${ATC_VERIFICADO} y el link está arriba. Si ves `
          + 'algo distinto de lo que publican hoy, escribinos y lo corregimos.',
      },
      {
        question: '¿El cargo de servicio de Vibe es en pesos o en dólares?',
        answer:
          'En pesos, como porcentaje de la seña. El abono de ATC es en dólares, así que no se restan '
          + 'directo sin meter un tipo de cambio en el medio.',
      },
    ],
  },
  {
    slug: 'cuanto-cobra-mercadopago-por-una-sena',
    tutorial: { slug: 'configuracion-del-complejo', texto: 'Dónde se conecta Mercado Pago y se define la seña' },
    title: 'Cuánto te cobra MercadoPago por cobrar una seña',
    seoTitle: 'Cuánto cobra MercadoPago por una seña, según tu provincia',
    /* Medida en pixeles, no en caracteres: 768px sobre un limite de 920. El margen
       importa porque el texto se arma desde el dataset, asi que si MercadoPago
       mueve una tasa cambia el largo. */
    metaDescription:
      `MercadoPago se queda con ${porcentaje(MINIMO)} a ${porcentaje(MAXIMO)} más IVA de cada seña, `
      + 'según tu provincia y tu plazo. La tabla oficial, completa.',
    excerpt:
      'Cobraste una seña y entró menos. Cuánto se queda MercadoPago según tu provincia y tu plazo, y '
      + 'lo que la tabla no dice.',
    respuesta:
      `MercadoPago se queda con entre ${porcentaje(MINIMO)} y ${porcentaje(MAXIMO)} de cada pago, más `
      + 'IVA, cuando cobrás una seña con Checkout. El número exacto sale de dos cosas: la provincia de '
      + 'tu domicilio registrado y cuántos días esperás para tener la plata disponible. Cobrar al '
      + `instante en Buenos Aires cuesta ${porcentaje(BA_INMEDIATA)}. Esperar 35 días en Chaco cuesta `
      + `${porcentaje(CHACO_LENTA)}. Costos vigentes desde el ${fechaEnTexto(VIGENTE_DESDE)}.`,
    datePublished: '2026-08-24',
    fuente: { nombre: 'MercadoPago, costos de Checkout', url: FUENTE },
    bloques: [
      {
        tipo: 'tabla',
        heading: 'Cuánto cobra según tu provincia y tu plazo',
        intro:
          'MercadoPago parte el país en nueve tarifas. La provincia que cuenta es la del domicilio '
          + 'registrado en tu cuenta: no la del complejo ni la del que reserva.',
        nota:
          'Ningún porcentaje incluye IVA ni retenciones. Con tarjetas de crédito extranjeras y tarjeta '
          + `Sucrédito, el costo sube ${RECARGO_TARJETA_EXTRANJERA} puntos.`,
        columnas: ['Provincia', ...PLAZOS],
        filas: filasDeCostos,
      },
      {
        tipo: 'parrafos',
        heading: 'Qué comprás cuando elegís el plazo',
        parrafos: [
          'El plazo no cambia cuándo cobrás: cambia cuándo podés usar la plata. El cliente paga el día '
            + 'que reserva. Lo que elegís es cuánto tiempo MercadoPago retiene ese dinero antes de '
            + 'dejártelo disponible.',
          'Entre cobrar al instante y esperar 35 días hay poco más de cinco puntos de la seña. En un '
            + 'complejo que cobra señas todos los días, eso deja de ser un detalle muy rápido.',
          'La pregunta útil no es cuál es más barato: siempre es el plazo más largo. Es si podés operar '
            + 'con la plata entrando 35 días después de la reserva. Si al profe le pagás el lunes, no '
            + 'podés.',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Cuánto te queda de una seña real',
        parrafos: [
          `Una seña de ${pesos(e.sena)} en Buenos Aires. Al instante, MercadoPago descuenta `
            + `${porcentaje(BA_INMEDIATA)}: ${pesos(e.inmediata.descuento)}, y te quedan `
            + `${pesos(e.inmediata.neto)}. A 35 días descuenta ${porcentaje(BA_LENTA)}: `
            + `${pesos(e.masLenta.descuento)}, y te quedan ${pesos(e.masLenta.neto)}.`,
          `Arriba de eso va el IVA sobre la comisión, del ${IVA}%. Si sos responsable inscripto, es `
            + 'crédito fiscal y lo recuperás. Si sos monotributista, no: es costo real, y la comisión '
            + 'te termina saliendo alrededor de un quinto más de lo que dice la tabla.',
          'Es la parte que casi ninguna comparación de plataformas incluye, y la que explica por qué lo '
            + 'que te entra nunca coincide con el porcentaje que leíste. Si además estás eligiendo '
            + 'plataforma, mirá <a href="/guias/cuanto-cuesta-un-sistema-de-reservas-para-canchas">cuánto '
            + 'cuesta un sistema de reservas para canchas</a>, con números reales.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Lo que la tabla oficial no dice',
        intro: 'Tres cosas que no están en el porcentaje y sí en tu resumen:',
        items: [
          `El IVA sobre la comisión. Los porcentajes publicados son sin IVA, y el IVA de la comisión es `
            + `${IVA}%.`,
          'Las retenciones impositivas, que dependen de tu condición fiscal y de tu provincia, no de '
            + 'MercadoPago.',
          `El recargo de ${RECARGO_TARJETA_EXTRANJERA} puntos con tarjetas de crédito extranjeras y `
            + 'tarjeta Sucrédito, que en un complejo con turistas deja de ser un caso raro.',
        ],
      },
    ],
    faq: [
      {
        question: '¿La comisión de MercadoPago la paga el complejo o el cliente?',
        answer:
          'La paga quien cobra: el complejo. MercadoPago la descuenta antes de que el pago llegue a tu '
          + 'cuenta, así que nunca la ves salir: ves entrar menos. Si cobrás con Vibe, el pago online es '
          + 'la seña más el cargo de servicio que paga el cliente, y MercadoPago calcula su porcentaje '
          + 'sobre ese total antes de separar la parte de Vibe. Vibe, en cambio, no le cobra nada al '
          + 'complejo.',
      },
      {
        question: '¿Puedo cambiar el plazo de acreditación después?',
        answer:
          'Sí. Se cambia en la sección de costos y cuotas de tu cuenta de MercadoPago, cuando quieras. '
          + 'Aplica a los cobros nuevos, no a los que ya están en curso.',
      },
      {
        question: '¿Por qué mi provincia paga distinto que la de al lado?',
        answer:
          'Porque la diferencia es impositiva, no comercial: cada provincia aplica su esquema y '
          + 'MercadoPago lo traslada al costo publicado. Por eso hay nueve tarifas y no una. Entre el '
          + `grupo más caro y el más barato hay ${BRECHA_INMEDIATA.toFixed(2).replace('.', ',')} puntos `
          + 'en la acreditación inmediata.',
      },
      {
        question: '¿Es lo mismo si cobro con link de pago o con QR?',
        answer:
          'No. Cada producto de MercadoPago tiene su tabla. Esta es la de Checkout, que es lo que usa '
          + 'una seña pagada desde la página del complejo; link de pago, QR y Point publican costos '
          + 'distintos. Y si cobrás en el mostrador con QR o con tarjeta y lo registrás en la caja de '
          + 'Vibe, Vibe no suma nada: pagás solo lo que cobre el medio que usaste.',
      },
      {
        question: '¿Cuándo cambian estos porcentajes?',
        answer:
          'Cuando MercadoPago los actualiza, sin aviso y sin frecuencia fija. Los de esta página rigen '
          + `desde el ${fechaEnTexto(VIGENTE_DESDE)}. Antes de decidir el plazo, confirmalos en tu `
          + 'cuenta: ahí está la tarifa que te aplican a vos.',
      },
    ],
  },
  {
    slug: 'cuanto-cuesta-un-sistema-de-reservas-para-canchas',
    verComparaciones: true,
    tutorial: { slug: 'precios-por-horario', texto: 'Cómo se cargan los precios por horario en el panel' },
    title: 'Cuánto cuesta de verdad un sistema de reservas para canchas',
    seoTitle: 'Cuánto cuesta un sistema de reservas de canchas, con números',
    metaDescription:
      `Abono fijo desde ${dolares(ATC_PLANES[0].mensual)} por mes o un cargo por reserva que paga el `
      + 'cliente: los dos modelos con números reales, según tu volumen.',
    excerpt:
      'El precio de lista no dice nada sin tu volumen. La misma cuota sale '
      + `${dolares(POR_RESERVA_POCAS)} o ${dolares(POR_RESERVA_MUCHAS)} por reserva según cuántas hagas.`,
    respuesta:
      `En Argentina hay dos modelos. Un abono mensual fijo, de ${dolares(ATC_PLANES[0].mensual)} a `
      + `${dolares(ATC_PLANES.at(-1)!.mensual)} según cuántas canchas tengas, que se paga haya reservas `
      + 'o no. O un cargo por reserva, que no le cobra nada fijo al complejo. Cuál te conviene depende '
      + `de una sola cosa: cuántos turnos hacés por mes. El mismo abono de ${dolares(ATC_PLANES[0].mensual)} `
      + `te sale ${dolares(POR_RESERVA_POCAS)} por reserva si hacés ${VOLUMENES[0]} al mes, y `
      + `${dolares(POR_RESERVA_MUCHAS)} si hacés ${VOLUMENES.at(-1)}.`,
    datePublished: '2026-08-24',
    fuente: { nombre: 'ATC Sports, precios y planes', url: ATC_FUENTE, nofollow: true },
    bloques: [
      {
        tipo: 'tabla',
        heading: 'Lo que te sale cada reserva con un abono fijo',
        intro:
          'La cuota dividida por los turnos del mes. Es la cuenta que ninguna página de precios hace, '
          + 'y la única que te dice si te conviene. No incluye la comisión de MercadoPago: esa se paga '
          + 'aparte, a quien recibe la plata, con cualquier plataforma que cobre online.',
        nota:
          'Precios de lista de ATC Sports en dólares —así los publican fuera de Argentina, y es el '
          + `precio que no se mueve solo con el tipo de cambio—, pagando mes a mes, verificados el `
          + `${ATC_VERIFICADO}. Los publican ellos y los pueden cambiar cuando quieran.`,
        columnas: columnasDeCostoPorReserva,
        filas: filasDeCostoPorReserva,
      },
      {
        tipo: 'parrafos',
        heading: 'Por qué el abono castiga al complejo chico',
        parrafos: [
          `Entre el volumen más bajo y el más alto de esa tabla hay ${VECES} veces de diferencia por `
            + 'reserva, con la misma cuota. El abono no sabe si tuviste un mes bueno o uno malo: se '
            + 'paga igual.',
          'Y pega donde más duele, porque los meses malos son justo cuando menos margen tenés. Un enero '
            + 'flojo o dos semanas de lluvia no bajan la cuota, y del lado de los ingresos lo único que '
            + 'mueve la aguja es '
            + '<a href="/guias/como-llenar-los-horarios-vacios-de-un-complejo">llenar los horarios que '
            + 'quedan vacíos</a>.',
          'La contracara, que es real: el abono es previsible. Sabés exactamente cuánto pagás el mes '
            + 'que viene, y para presupuestar eso vale. Un cargo por reserva sube cuando te va bien.',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'El otro modelo: quién paga qué',
        parrafos: [
          'Vibe no cobra abono: el complejo no le paga nada fijo, haya reservas o no. Lo que hay es un '
            + `cargo de servicio del ${CARGO_SERVICIO_TEXTO} sobre la seña, con un mínimo de `
            + `${pesos(CARGO_MINIMO)}, que paga el cliente que reserva. Sobre una seña de `
            + `${pesos(SENA_DE_EJEMPLO)} son ${pesos(cargoPorReserva())}, y los pone él, no vos.`,
          'Ese cargo es lo único que cobra Vibe, y solo en una reserva online. La caja, la venta de '
            + 'productos con stock y lo que cobrás en el mostrador —efectivo, transferencia, débito, '
            + 'crédito o QR— no tienen ningún cargo de Vibe. Un abono como el de ATC también los '
            + 'incluye, así que en eso los dos modelos empatan.',
          'Los dos modelos ni siquiera están en la misma moneda: ATC publica en dólares y el cargo de '
            + 'Vibe es en pesos. Pasar uno al otro metería un tipo de cambio, que es un número más que '
            + 'se desactualiza solo. Así que lo que sigue compara cómo se reparte el costo entre '
            + 'complejo y cliente, no resta un número contra otro.',
          'Del lado del complejo: con los dos modelos pagás la comisión de MercadoPago sobre lo que se '
            + 'cobra online (mirá <a href="/guias/cuanto-cobra-mercadopago-por-una-sena">cuánto cobra '
            + 'MercadoPago por una seña</a> según tu provincia), no sobre el precio total de la cancha. '
            + 'Con Vibe, el pago online incluye el cargo de servicio, así que esa comisión se calcula '
            + 'sobre un poco más. Lo que queda de diferencia es el abono: con ATC, una cuota fija en '
            + 'dólares haya reservas o no; con Vibe, cero. Siempre.',
          'Del lado del cliente es al revés: con un abono paga la cancha y nada más; con un cargo por '
            + `reserva paga la cancha más ${pesos(cargoPorReserva())}. Ahí el abono le sale más barato `
            + 'a él.',
          'La salvedad que conviene tener presente: una cuota fija dividida por reservas se abarata sola '
            + 'con el volumen, hasta casi cero. Un cargo por reserva no baja nunca, porque es un '
            + 'porcentaje de la seña. A partir de cierto volumen, cualquier cuota fija sale más barata '
            + 'por turno que cualquier cargo por reserva, sea la empresa que sea y en la moneda que sea.',
          'Sin vueltas: el cargo por reserva le saca el costo fijo al complejo y se lo pasa al que '
            + 'reserva. Del lado del mostrador conviene siempre —cero es menos que cualquier abono, a '
            + 'cualquier volumen—. Del lado del cliente, depende de cuánto valga para él reservar y '
            + 'pagar desde el celular en vez de llamar.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Lo que ninguna página de precios te dice',
        intro: 'Tres cosas que no están en la lista y sí en tu cuenta a fin de mes:',
        items: [
          'La comisión de MercadoPago corre con los dos modelos. Si cobrás señas online, MercadoPago '
            + 'se lleva lo suyo, tengas abono o no.',
          'El abono suele ir escalonado por canchas: sumar una puede saltarte de plan sin que cambie '
            + 'nada más.',
          'El precio más bajo anunciado suele ser pagando el año por adelantado. Mes a mes cuesta más, '
            + 'y es el número a comparar si no vas a adelantar doce meses.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Conviene pagar el año por adelantado?',
        answer:
          `Sale más barato por mes: en ATC, el Plan Base pasa de ${dolares(ATC_PLANES[0].mensual)} a `
          + `${dolares(ATC_PLANES[0].anual)} pagando los doce juntos. Lo que comprás con ese descuento `
          + 'es quedarte un año. La pregunta real es qué pasa si a los tres meses te das cuenta de que '
          + 'no era para vos.',
      },
      {
        question: '¿Cuántas reservas por mes hace un complejo típico?',
        answer:
          'Depende demasiado de las canchas y de la ocupación como para dar un número honesto. Contá '
          + 'los turnos del último mes en tu cuaderno o planilla y usá ese número contra la tabla. Es '
          + 'tu dato, no el promedio de nadie.',
      },
      {
        question: '¿El cargo por reserva no espanta a los clientes?',
        answer:
          'Se ve antes de pagar, no después: el que llega a la pantalla de pago ya sabe cuánto es. Lo '
          + 'que puede frenarlo es el total, no una sorpresa. Y si cancela dentro del plazo, el cargo '
          + 'se le devuelve junto con la seña.',
      },
      {
        question: '¿Hay sistemas gratis de verdad?',
        answer:
          'Hay planes gratuitos con tope de canchas o de reservas, y pruebas por tiempo limitado. Antes '
          + 'de contarlos como gratis, mirá dos cosas: qué pasa cuando pasás el tope, y si cobrar señas '
          + 'online está incluido o es un extra. Vibe no le cobra nada al complejo, sin planes ni '
          + 'abono: el cargo de servicio lo paga el que reserva online, y la comisión de MercadoPago '
          + 'corre aparte, como con cualquiera.',
      },
      {
        question: '¿El cargo de Vibe baja si tengo mucho volumen?',
        answer:
          'No. Es un porcentaje fijo de la seña con un mínimo en pesos: no cambia con el volumen. Un '
          + 'abono, en cambio, se abarata por reserva cuantos más turnos hacés. Justo al revés.',
      },
    ],
  },
  {
    slug: 'como-llenar-los-horarios-vacios-de-un-complejo',
    tutorial: { slug: 'panel-de-control', texto: 'El mapa de ocupación que muestra tus horas muertas' },
    title: 'Cómo llenar los horarios vacíos de tu complejo',
    seoTitle: 'Horarios vacíos en tu complejo deportivo: cómo llenarlos',
    /* Esta guía es el género donde se inventan estadísticas ("+30% de ocupación
       en 60 días"). No hay ninguna propia que mostrar acá, así que lo único que
       se afirma es el mecanismo. Los porcentajes que sí aparecen son los del
       cargo y los de MercadoPago, y salen de las constantes, no del teclado. */
    metaDescription:
      'Las horas muertas no se llenan con promociones: se llenan con diagnóstico, reserva sin teléfono '
      + 'y seña. Cinco pasos, sin estadísticas inventadas.',
    excerpt:
      'El martes a las diez vacío no es mala suerte. Qué mirar, en qué orden, y qué hacer hoy con cada '
      + 'franja que quedó libre.',
    respuesta:
      'No se llenan con una promoción: se llenan sacando del medio, una por una, las cosas que frenan '
      + 'una reserva. Primero mirás una semana de datos reales para saber qué horas están vacías de '
      + 'verdad y en qué cancha. Después hacés que se pueda reservar sin hablar con nadie, a cualquier '
      + 'hora. Después pedís seña, que es lo que convierte un "te aviso" en un turno. Después ponés ese '
      + 'link donde la gente ya te busca. Y al final volvés sobre los que ya vinieron, que son los más '
      + 'baratos de traer. Ninguno de los cinco pasos necesita Vibe: necesitan estar hechos.',
    datePublished: '2026-09-15',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Primero: cuáles son tus horas muertas de verdad',
        parrafos: [
          '"A la mañana está vacío" es una impresión, no un dato, y casi siempre es media verdad: lo '
            + 'que está vacío es el martes a las diez, no la mañana. La diferencia importa. Una '
            + 'promoción para toda la mañana regala descuento en los turnos que se vendían igual, y '
            + 'deja el martes como estaba.',
          'Lo que hace falta es una semana entera, anotada hora por hora y cancha por cancha. Sirve un '
            + 'cuaderno. Lo que no sirve es el recuerdo: el recuerdo guarda los sábados llenos y se '
            + 'olvida de los martes.',
          'Después separá dos cosas que se mezclan siempre: los días de semana y el fin de semana. Un '
            + 'complejo puede tener el sábado casi lleno y el miércoles casi vacío, y dar un promedio '
            + 'decente que no describe ninguno de los dos. La ocupación promedio es el número que menos '
            + 'te sirve.',
          'Si usás Vibe, el panel ya arma parte de eso: el mapa de calor cruza hora del día contra día '
            + 'de la semana y marca el pico, el calendario muestra el día cancha por cancha, y el '
            + 'reporte mensual abre las reservas y la plata por cancha. Nada de eso te dice qué hacer; '
            + 'te dice dónde mirar.',
          'Qué hacer hoy: anotá una semana completa, hora por hora y cancha por cancha, y marcá las '
            + 'franjas vacías. Esas franjas son el problema, no "la mañana".',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Que se pueda reservar sin que nadie atienda',
        parrafos: [
          'Una parte de los turnos vacíos no está vacía porque nadie los quiera: está vacía porque '
            + 'cuando alguien quiso reservar, no había con quién hablar. El mensaje entra a las once de '
            + 'la noche, lo contestás a las nueve de la mañana, y a esa hora esa persona ya jugó en '
            + 'otro lado.',
          'Cuántas reservas se pierden así no lo sabemos y no lo vamos a inventar: depende de tu '
            + 'complejo y de tu horario. Lo que sí podés medir, en tu propio teléfono, es cuántos '
            + 'mensajes de reserva te entraron fuera del horario en el que contestás.',
          'El mecanismo no tiene vuelta. Si hay una página donde se ve la grilla libre y se reserva sin '
            + 'esperar respuesta, la reserva entra cuando entra y el turno deja de depender de que '
            + 'estés despierto. El trabajo real no es el software: es tener la grilla cargada de '
            + 'verdad, con horarios y precios al día. Una página que muestra libre un horario que no lo '
            + 'está hace más daño que no tener página.',
          'Qué hacer hoy: contá en tu WhatsApp los mensajes de reserva que entraron fuera de tu horario '
            + 'esta semana. Ese número es tuyo, es real, y es el tamaño de lo que estás perdiendo.',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'La seña es lo que separa un turno de un "te aviso"',
        parrafos: [
          'Reservar sin poner nada no cuesta nada, y lo que no cuesta nada se cancela sin avisar. El '
            + 'que puso plata, viene. Esa es toda la función de la seña: no es financiamiento, es un '
            + 'filtro.',
          'Y filtra para los dos lados. Al que iba a ir no lo espanta, porque ya pensaba pagar. Al que '
            + 'estaba tanteando lo saca de la grilla ahora, cuando todavía podés vender ese turno, y no '
            + 'a las ocho de la noche, cuando ya no se lo vendés a nadie.',
          'Con Vibe, el complejo no paga abono ni costo fijo. Al que reserva se le suma un cargo de '
            + `servicio del ${CARGO_SERVICIO_TEXTO} sobre la seña, con un mínimo de ${pesos(CARGO_MINIMO)}: `
            + `sobre una seña de ${pesos(SENA_DE_EJEMPLO)} son ${pesos(cargoPorReserva())}, y los paga `
            + 'él, no vos. Lo ve antes de pagar. Si cancela dentro de la ventana que configuraste, '
            + 'MercadoPago le devuelve la seña solo, y el cargo de servicio vuelve con ella.',
          'Aparte está la comisión de MercadoPago, que esa sí la paga el complejo: se descuenta del '
            + `pago online antes de que llegue a tu cuenta, y va ${RANGO_NACIONAL} más IVA según tu `
            + 'provincia y el plazo que elijas. La tabla completa está en '
            + '<a href="/guias/cuanto-cobra-mercadopago-por-una-sena">cuánto cobra MercadoPago por una '
            + 'seña</a>.',
          'Qué hacer hoy: definí un porcentaje de seña y una ventana de cancelación, y escribilos donde '
            + 'la gente reserva. Una regla escrita se discute mucho menos que una que hay que explicar '
            + 'por teléfono cada vez.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Dónde te tienen que encontrar',
        intro:
          'El que busca cancha un viernes a la tarde no entra a tu web: busca en Google, mira Instagram '
          + 'o manda un WhatsApp. En los tres tiene que estar el mismo link, y tiene que llevar a la '
          + 'grilla donde se reserva, no a una portada.',
        items: [
          'La ficha de Google de tu complejo. Es lo que aparece cuando alguien busca canchas más el '
            + 'nombre del barrio, y suele estar a medias: sin horarios, sin fotos, con el teléfono como '
            + 'único contacto. El campo del sitio web tiene que apuntar a donde se reserva.',
          'La bio de Instagram. Un link: el de reservar. Si hay cinco, el que importa se pierde; y si el '
            + 'único llamado es "mandanos un DM", volviste a depender de que alguien conteste.',
          'El WhatsApp del complejo. El mensaje automático de bienvenida es el lugar más barato que '
            + 'existe para poner el link: contesta solo, a cualquier hora, a alguien que ya te está '
            + 'escribiendo.',
          'El mismo link en los tres. Si Google, Instagram y WhatsApp llevan a tres lugares distintos, '
            + 'no sabés cuál funciona y el que reserva no sabe cuál es el oficial.',
          'Qué hacer hoy: abrí los tres y fijate si llevan al mismo lado. El que no tenga link es, hoy, '
            + 'el que te está mandando gente al teléfono.',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Los que ya vinieron son los más baratos de traer',
        parrafos: [
          'Conseguir un cliente nuevo cuesta plata o cuesta tiempo. Traer de vuelta a uno que ya jugó '
            + 'en tu cancha, que sabe dónde queda y cuánto sale, cuesta un mensaje.',
          'La lista ya la tenés, aunque esté repartida entre el cuaderno y el historial de WhatsApp. Lo '
            + 'que falta es ordenarla en tres montones: el que viene todas las semanas, el que vino '
            + 'varias veces y hace rato que no aparece, y el que reservó y no se presentó. Son tres '
            + 'conversaciones distintas y merecen tres mensajes distintos.',
          'Al habitué no hay que venderle nada: hay que ofrecerle el turno fijo. Al que dejó de venir '
            + 'se le escribe una vez, sin promoción, preguntando si pasó algo, y la respuesta suele '
            + 'contarte algo de tu complejo que no sabías. Al que no se presentó se le pide seña la '
            + 'próxima vez, y listo.',
          'En Vibe la ficha de cada cliente guarda sus reservas, sus ausencias y su asistencia, y el '
            + 'panel separa a los nuevos de los recurrentes y muestra los que más vienen. Sin Vibe, la '
            + 'misma información está en tu cuaderno: da más trabajo sacarla, no es imposible.',
          'Si estás eligiendo con qué herramienta hacer todo esto, los dos modelos de precio que hay en '
            + 'Argentina están abiertos con números en '
            + '<a href="/guias/cuanto-cuesta-un-sistema-de-reservas-para-canchas">cuánto cuesta un '
            + 'sistema de reservas para canchas</a>.',
          'Qué hacer hoy: sacá de tu historial a los que venían seguido y hace rato que no aparecen, y '
            + 'escribiles uno por uno. Sin lista de difusión: un mensaje reenviado se nota y no se '
            + 'contesta.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Cuánto tiempo hay que medir antes de tocar algo?',
        answer:
          'Una semana entera y de corrido, como mínimo, porque necesitás los siete días. Si podés '
          + 'anotar un mes, mejor: recién ahí ves si el martes flojo es todos los martes o fue ese '
          + 'martes. Lo que no sirve es medir tres días buenos y decidir con eso.',
      },
      {
        question: '¿Pedir seña no me hace perder reservas?',
        answer:
          'Pierde las que se iban a caer igual, que es distinto. Lo que cambia es cuándo te enterás de '
          + 'que el turno se cayó: con seña, al momento de reservar, porque el que no pensaba ir no '
          + 'llega a pagar; sin seña, cuando el turno ya pasó y la cancha quedó vacía.',
      },
      {
        question: '¿Esto sirve si no uso ningún sistema de reservas?',
        answer:
          'Sí. Los cinco pasos son de gestión, no de software: medir una semana, tener una forma de '
          + 'reservar que no dependa de que alguien conteste, pedir seña, poner el mismo link en los '
          + 'tres lugares donde te buscan y volver sobre los que ya vinieron. Un sistema los hace más '
          + 'rápido y te ahorra transcribir; ninguno de los cinco lo inventa.',
      },
      {
        question: '¿Por dónde empiezo si tengo una sola cancha?',
        answer:
          'Por el segundo paso. Con una cancha el diagnóstico lo tenés en la cabeza y no te vas a '
          + 'equivocar demasiado, así que lo que más rinde es que se pueda reservar sin que atiendas: '
          + 'con una sola cancha, cada turno que se pierde por un mensaje sin contestar es un pedazo '
          + 'grande de tu día.',
      },
      {
        question: '¿Cómo sé si lo que hice funcionó?',
        answer:
          'Comparando las mismas franjas contra la semana que anotaste antes de tocar nada, no la '
          + 'ocupación general. Si moviste el martes a la mañana, mirá el martes a la mañana. El '
          + 'promedio del complejo se mueve por el clima, los feriados y el mes del año: sirve para '
          + 'todo menos para saber si tu cambio hizo algo.',
      },
    ],
  },
];

/* La fecha sale del contenido de cada guia. `datePublished` queda afuera del
   hash: es fijo, meterlo no cambiaria nada, y dejarlo afuera deja mas claro que
   lo que se mide es lo que la guia DICE. */
export const guias: Guia[] = contenido.map(g => {
  const { datePublished, ...loQueSeMide } = g;
  return { ...g, dateModified: fechaDeContenido(g.slug, loQueSeMide) };
});
guardarFechas();

export const getGuia = (slug: string) => guias.find(g => g.slug === slug);

/** Las comparaciones con competidores, menos la que se está leyendo. */
export const comparaciones = (excepto?: string) =>
  guias.filter(g => g.comparacion && g.slug !== excepto);

/** Fecha de la guía más nueva, para el índice. */
export const ULTIMA_ACTUALIZACION = guias
  .map(g => g.dateModified)
  .sort()
  .at(-1) ?? VIGENTE_DESDE;
