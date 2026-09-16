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
    slug: 'vibe-o-atc-sports-en-que-se-diferencian',
    tutorial: { slug: 'grilla-de-reservas', texto: 'Cómo se ve la grilla de reservas de Vibe' },
    title: 'Vibe o ATC Sports: en qué se diferencian',
    seoTitle: 'Vibe o ATC Sports: en qué se diferencian',
    metaDescription:
      'Comparación honesta entre Vibe y ATC Sports para complejos deportivos: modelo de cobro, '
      + 'qué trae cada uno y en qué casos conviene cada uno.',
    excerpt:
      'Los dos resuelven la reserva online. La diferencia está en quién paga, cuándo, y en '
      + 'cuánto abarca cada uno fuera de la cancha.',
    respuesta:
      'La diferencia principal no está en las funciones de reserva, que las dos plataformas cubren: '
      + `está en el modelo de cobro. ATC Sports cobra un abono mensual fijo en dólares, desde ${dolares(ATC_PLANES[0].mensual)} `
      + `hasta ${dolares(ATC_PLANES.at(-1)!.mensual)} según cuántas canchas tengas, y se paga haya reservas o no. `
      + 'Vibe no cobra abono: el complejo no le paga nada a la plataforma, y el cargo de servicio lo paga el '
      + 'cliente sobre la seña. La segunda diferencia es el alcance: ATC publica funciones que Vibe no tiene, '
      + 'como control de caja e inventario, integración con grabación de partidos y banners QR.',
    datePublished: '2026-09-16',
    fuente: { nombre: 'ATC Sports, software de gestión deportiva', url: ATC_FUENTE, nofollow: true },
    bloques: [
      {
        tipo: 'lista',
        heading: 'Lo que hacen las dos',
        intro:
          'Conviene empezar por acá, porque es la mayor parte. En estas cosas elegir una u otra no cambia '
          + 'lo que vas a poder hacer:',
        items: [
          'Reserva online, sin que tengas que contestar un mensaje',
          'Grilla de turnos con todas las canchas del día',
          'Datos de cada cancha y de cada cliente',
          'Reportes de lo que facturaste',
          'Varios usuarios y acceso desde el celular',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'La diferencia real: quién paga y cuándo',
        parrafos: [
          'ATC cobra un abono mensual por complejo, en dólares, escalonado por cantidad de canchas. Es un costo '
            + 'previsible: sabés lo que vas a pagar el mes que viene, tengas un enero flojo o un agosto lleno. '
            + `También ofrece ${ATC_PRUEBA_GRATIS_DIAS} días de prueba gratis y un ${ATC_DESCUENTO_ANUAL_PORCENTAJE} por ciento `
            + 'de descuento si pagás el año por adelantado.',
          'Vibe no tiene abono. El complejo no le paga nada a la plataforma; lo que se cobra es un cargo de '
            + 'servicio sobre la seña, y lo paga el cliente que reserva. Eso significa que un mes sin reservas '
            + 'no te cuesta nada, y también que tu cliente ve un importe un poco más alto al reservar.',
          'Ninguno de los dos modelos es mejor en abstracto. El abono conviene cuando el volumen es alto y '
            + 'previsible, porque se reparte entre muchas reservas. El cargo por reserva conviene cuando el '
            + 'volumen es bajo o irregular, porque no hay nada que pagar cuando no pasa nada.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Lo que ATC hace y Vibe no',
        intro:
          'Está publicado en su página y es cierto. Si necesitás alguna de estas cosas, Vibe no te sirve y '
          + 'ATC sí:',
        items: [
          'Control de caja e inventario, para el bar o la venta de artículos',
          'Integración con grabación de partidos',
          'Banners QR y paquetes digitales personalizados',
          'Sitio web propio del complejo, más allá de la página de reservas',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Cómo elegir sin probar los dos',
        parrafos: [
          'Si vendés en el bar, manejás stock o querés grabar los partidos, la decisión ya está tomada y no '
            + 'es Vibe. Ese es el corte más limpio.',
          'Si lo que necesitás es que la gente reserve y pague la seña sola, la pregunta pasa a ser cuánto '
            + 'volumen tenés y quién querés que soporte el costo. Un abono fijo dividido por muchas reservas '
            + 'termina siendo barato por turno; dividido por pocas, caro. Esa cuenta está hecha, con la tabla '
            + 'completa, en la guía de cuánto cuesta un sistema de reservas.',
          'Y si estás arrancando o tu temporada baja es muy baja, el argumento más fuerte a favor de no tener '
            + 'abono no es el precio: es que no tenés que acertar el pronóstico. Podés equivocarte con la '
            + 'demanda sin que eso te cueste plata todos los meses.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Puedo migrar de ATC a Vibe sin perder mis reservas?',
        answer:
          'Las reservas futuras se cargan a mano desde la grilla, igual que una reserva telefónica. El '
          + 'historial viejo no se importa: la base de clientes de Vibe se arma sola con las reservas nuevas.',
      },
      {
        question: '¿Vibe tiene prueba gratis?',
        answer:
          'No hace falta: como no hay abono, no hay nada que probar antes de pagar. Cargás el complejo y el '
          + 'único costo aparece cuando alguien reserva y paga una seña.',
      },
      {
        question: '¿El cargo de servicio lo puedo absorber yo en vez del cliente?',
        answer:
          'El cargo se le suma al cliente al pagar la seña. Si preferís que no lo vea, lo que podés hacer es '
          + 'ajustar el precio de la cancha para compensarlo, pero eso es una decisión de precio tuya.',
      },
      {
        question: '¿Esta comparación está actualizada?',
        answer:
          `Los datos de ATC se leyeron de su página el ${ATC_VERIFICADO} y el enlace a la fuente está arriba. `
          + 'Si ves algo que no coincide con lo que ellos publican hoy, escribinos y lo corregimos.',
      },
    ],
  },
  {
    slug: 'cuanto-cobra-mercadopago-por-una-sena',
    tutorial: { slug: 'configuracion-del-complejo', texto: 'Dónde se conecta Mercado Pago y se define la seña' },
    title: 'Cuánto cobra MercadoPago por cobrar una seña',
    seoTitle: 'Cuánto cobra MercadoPago por una seña | Con fuente oficial',
    /* Medida en pixeles, no en caracteres: 772px sobre un limite de 920. El margen
       importa porque el texto se arma desde el dataset, asi que si MercadoPago
       mueve una tasa cambia el largo. */
    metaDescription:
      `MercadoPago cobra de ${porcentaje(MINIMO)} a ${porcentaje(MAXIMO)} más IVA por una seña, según tu provincia. `
      + 'Tabla completa y actualizada, con fuente oficial.',
    excerpt:
      'El costo cambia según la provincia y según cuántos días esperes la plata. '
      + 'Los nueve grupos de tarifas, y lo que la tabla oficial no te dice.',
    respuesta:
      `MercadoPago cobra entre ${porcentaje(MINIMO)} y ${porcentaje(MAXIMO)} de la seña, más IVA, cuando cobrás con Checkout. `
      + 'El porcentaje exacto sale de dos variables: en qué provincia está registrado tu domicilio y cuántos días '
      + `esperás para tener la plata disponible. Cobrar al instante en Buenos Aires cuesta ${porcentaje(BA_INMEDIATA)}. `
      + `Esperar 35 días en Chaco cuesta ${porcentaje(CHACO_LENTA)}. Costos vigentes desde el ${fechaEnTexto(VIGENTE_DESDE)}.`,
    datePublished: '2026-08-24',
    fuente: { nombre: 'MercadoPago, costos de Checkout', url: FUENTE },
    bloques: [
      {
        tipo: 'tabla',
        heading: 'Cuánto cobra según tu provincia y tu plazo',
        intro:
          'MercadoPago agrupa el país en nueve tarifas distintas. La provincia que cuenta es la del domicilio '
          + 'registrado en tu cuenta, no la del complejo ni la del que reserva.',
        nota:
          'Ninguno de estos porcentajes incluye IVA ni retenciones. Con tarjetas de crédito extranjeras y '
          + `tarjeta Sucrédito, el costo sube ${RECARGO_TARJETA_EXTRANJERA} puntos.`,
        columnas: ['Provincia', ...PLAZOS],
        filas: filasDeCostos,
      },
      {
        tipo: 'parrafos',
        heading: 'Qué estás comprando cuando elegís el plazo',
        parrafos: [
          'El plazo no cambia cuándo cobrás, cambia cuándo podés usar la plata. La reserva se paga igual el día '
          + 'que el cliente reserva. Lo que elegís es cuánto tiempo MercadoPago retiene ese dinero antes de '
          + 'dejarlo disponible en tu cuenta.',
          'La diferencia entre cobrar al instante y esperar 35 días es de poco más de cinco puntos de la seña. En un '
          + 'complejo que factura señas todos los días, esa diferencia deja de ser un detalle bastante rápido.',
          'La pregunta útil no es cuál es más barato, porque siempre es el plazo más largo. Es si podés operar '
          + 'con la plata entrando 35 días después de la reserva. Si tenés que pagarle al profe el lunes, no podés.',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Cuánto te queda de una seña real',
        parrafos: [
          `Una seña de ${pesos(e.sena)} en Buenos Aires. Con acreditación inmediata, MercadoPago descuenta `
          + `${porcentaje(BA_INMEDIATA)}, o sea ${pesos(e.inmediata.descuento)}, y te quedan ${pesos(e.inmediata.neto)}. `
          + `Con acreditación a 35 días descuenta ${porcentaje(BA_LENTA)}, o sea ${pesos(e.masLenta.descuento)}, `
          + `y te quedan ${pesos(e.masLenta.neto)}.`,
          `A esos descuentos hay que sumarles el IVA sobre la comisión, que es del ${IVA}%. Si sos responsable `
          + 'inscripto, ese IVA es crédito fiscal y lo recuperás. Si sos monotributista, no lo recuperás y es '
          + 'costo real: la comisión te termina saliendo alrededor de un quinto más de lo que dice la tabla.',
          'Es la parte que casi ninguna comparación de plataformas incluye, y es la que explica por qué el '
          + 'número que te cierra en la cuenta nunca coincide con el porcentaje que leíste. Si además estás '
          + 'evaluando qué plataforma usar, conviene mirar '
          + '<a href="/guias/cuanto-cuesta-un-sistema-de-reservas-para-canchas">cuánto cuesta un sistema de reservas para canchas</a> '
          + 'con números reales.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Lo que la tabla oficial no dice',
        intro: 'Tres cosas que no aparecen en el porcentaje y sí aparecen en tu resumen.',
        items: [
          `El IVA sobre la comisión. Los porcentajes publicados son sin IVA, y el IVA de la comisión es ${IVA}%.`,
          'Las retenciones impositivas, que dependen de tu condición fiscal y de tu provincia, no de MercadoPago.',
          `El recargo de ${RECARGO_TARJETA_EXTRANJERA} puntos en tarjetas de crédito extranjeras y tarjeta Sucrédito, que en un complejo con `
          + 'turistas deja de ser un caso raro.',
        ],
      },
    ],
    faq: [
      {
        question: '¿El costo de MercadoPago lo paga el complejo o el cliente?',
        answer:
          'Lo paga quien cobra, o sea el complejo. Se descuenta de la seña antes de que el dinero llegue a tu '
          + 'cuenta, así que nunca lo ves salir: ves entrar menos. Es distinto del cargo de servicio de Vibe, '
          + 'que se le suma al cliente al reservar y no sale de lo que cobrás vos.',
      },
      {
        question: '¿Puedo cambiar el plazo de acreditación después de configurarlo?',
        answer:
          'Sí. El plazo se configura en la sección de costos y cuotas de tu cuenta de MercadoPago y se puede '
          + 'cambiar cuando quieras. El cambio aplica a los cobros nuevos, no a los que ya están en curso.',
      },
      {
        question: '¿Por qué mi provincia paga distinto que la de al lado?',
        answer:
          'Porque la diferencia es impositiva, no comercial. Cada provincia aplica su propio esquema sobre la '
          + 'operación, y MercadoPago lo traslada al costo publicado. Por eso hay nueve tarifas y no una: la '
          + `diferencia entre el grupo más caro y el más barato es de ${BRECHA_INMEDIATA.toFixed(2).replace('.', ',')} puntos en la acreditación inmediata.`,
      },
      {
        question: '¿Estos costos son los mismos si cobro con link de pago o con QR?',
        answer:
          'No. Cada producto de MercadoPago tiene su propia tabla. Estos son los de Checkout, que es lo que usa '
          + 'una reserva pagada desde la página del complejo. Link de pago, QR y Point publican costos distintos.',
      },
      {
        question: '¿Cuándo cambian estos porcentajes?',
        answer:
          'Cuando MercadoPago los actualiza, sin aviso previo y sin una frecuencia fija. Los de esta página '
          + `están vigentes desde el ${fechaEnTexto(VIGENTE_DESDE)}. Antes de tomar una decisión sobre el plazo conviene `
          + 'confirmarlos en tu cuenta, que es donde figura la tarifa que efectivamente te aplican.',
      },
    ],
  },
  {
    slug: 'cuanto-cuesta-un-sistema-de-reservas-para-canchas',
    tutorial: { slug: 'precios-por-horario', texto: 'Cómo se cargan los precios por horario en el panel' },
    title: 'Cuánto cuesta un sistema de reservas para canchas',
    seoTitle: 'Cuánto cuesta un sistema de reservas de canchas | Precios reales',
    metaDescription:
      `Abono fijo desde ${dolares(ATC_PLANES[0].mensual)} por mes, o un cargo por reserva: comparamos `
      + 'ambos modelos con números reales para saber cuál conviene.',
    excerpt:
      'El precio de lista no dice nada sin el volumen. La misma cuota sale '
      + `${dolares(POR_RESERVA_POCAS)} o ${dolares(POR_RESERVA_MUCHAS)} por reserva según cuántas hagas.`,
    respuesta:
      `En Argentina hay dos modelos. Un abono mensual fijo, que arranca en ${dolares(ATC_PLANES[0].mensual)} `
      + `y llega a ${dolares(ATC_PLANES.at(-1)!.mensual)} según cuántas canchas tengas, y se paga haya reservas o no. `
      + `O un cargo por reserva, que no cobra nada fijo. Cuál conviene depende de una sola cosa: cuántos `
      + `turnos hacés por mes. El mismo abono de ${dolares(ATC_PLANES[0].mensual)} sale ${dolares(POR_RESERVA_POCAS)} `
      + `por reserva si hacés ${VOLUMENES[0]} al mes, y ${dolares(POR_RESERVA_MUCHAS)} si hacés ${VOLUMENES.at(-1)}.`,
    datePublished: '2026-08-24',
    fuente: { nombre: 'ATC Sports, precios y planes', url: ATC_FUENTE, nofollow: true },
    bloques: [
      {
        tipo: 'tabla',
        heading: 'Lo que sale cada reserva con un abono fijo',
        intro:
          'La cuota dividida por la cantidad de turnos del mes. Es la cuenta que no aparece en '
          + 'ninguna página de precios, y es la única que te dice si te conviene.',
        nota:
          `Precios de lista de ATC Sports en dólares —así los publican fuera de Argentina, y es el precio `
          + `que no se mueve solo con el tipo de cambio—, pagando mes a mes, verificados el ${ATC_VERIFICADO}. `
          + 'Los publican ellos y los pueden cambiar cuando quieran.',
        columnas: columnasDeCostoPorReserva,
        filas: filasDeCostoPorReserva,
      },
      {
        tipo: 'parrafos',
        heading: 'Por qué el abono castiga al complejo chico',
        parrafos: [
          `Entre el volumen más bajo y el más alto de esa tabla hay ${VECES} veces de diferencia por reserva, `
          + 'con exactamente la misma cuota. El abono no sabe si tuviste un mes bueno o uno malo: se paga igual.',
          'Eso pega más fuerte de lo que parece, porque los meses malos son justo cuando menos margen tenés. '
          + 'Un enero flojo o dos semanas de lluvia no bajan la cuota, y del lado de los ingresos lo único que '
          + 'mueve la aguja es '
          + '<a href="/guias/como-llenar-los-horarios-vacios-de-un-complejo">llenar los horarios que quedan vacíos</a>.',
          'La contracara, que es real y conviene decirla: el abono es previsible. Sabés exactamente cuánto vas '
          + 'a pagar el mes que viene, y para presupuestar eso vale. Un cargo por reserva sube cuando te va bien.',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'El otro modelo, y quién paga qué',
        parrafos: [
          `Vibe no cobra abono: el complejo no le paga nada fijo a Vibe, haya reservas o no. Lo que hay es un `
          + `cargo de servicio del ${CARGO_SERVICIO_TEXTO} sobre la seña, con un mínimo de ${pesos(CARGO_MINIMO)}, `
          + `que se le suma al cliente que reserva. Sobre una seña de ${pesos(SENA_DE_EJEMPLO)} son `
          + `${pesos(cargoPorReserva())}, y los paga él, no el complejo.`,
          'Los dos modelos ni siquiera están en la misma moneda: ATC publica en dólares y el cargo de servicio '
          + 'de Vibe se cobra en pesos. Convertir uno al otro metería un tipo de cambio en el medio, y un tipo '
          + 'de cambio es un número más que se desactualiza solo, que es justo lo que esta página evita en todo '
          + 'lo demás. Así que lo que sigue compara cómo se reparte el costo entre complejo y cliente en cada '
          + 'modelo, no resta un número contra el otro.',
          'Del lado del complejo: con los dos modelos se paga la comisión de MercadoPago (mirá '
          + '<a href="/guias/cuanto-cobra-mercadopago-por-una-sena">cuánto cobra MercadoPago por una seña</a> '
          + 'según tu provincia), y en los dos casos es sobre la seña, no sobre el precio total de la cancha. '
          + 'Esa parte se cancela. Lo que queda de diferencia es el abono en sí: con ATC es una cuota fija en '
          + 'dólares que se paga haya reservas o no; con Vibe es cero, siempre.',
          `Del lado del cliente es al revés: con un abono paga el precio de la cancha y nada más, y con un cargo `
          + `por reserva paga el precio más ${pesos(cargoPorReserva())}. Ahí el abono le sale más barato a él.`,
          'Y la salvedad, que hay que decirla porque esta página no está para que gane Vibe: una cuota fija '
          + 'dividida por reservas —como el abono de ATC en la tabla de arriba— se abarata sola con el volumen, '
          + 'tendiendo a cero. Un cargo por reserva, en cambio, no baja nunca, porque es un porcentaje fijo de '
          + 'la seña. A partir de cierto volumen, cualquier cuota fija termina saliendo más barata por turno que '
          + 'cualquier cargo por reserva, sea cual sea la empresa y sea cual sea la moneda: es aritmética, no '
          + 'una opinión.',
          'Resumido sin vueltas: el modelo de cargo por reserva le saca el costo fijo al complejo y se lo pasa a '
          + 'quien reserva. Eso conviene siempre del lado del mostrador —pagar cero es menos que pagar cualquier '
          + 'abono, a cualquier volumen—. Del lado del cliente depende de cuánto valga para él reservar y pagar '
          + 'desde el celular en vez de por teléfono.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Lo que ninguna página de precios te dice',
        intro: 'Tres cosas que no están en la lista y sí están en tu cuenta a fin de mes.',
        items: [
          'La comisión de la pasarela de pago se paga con los dos modelos. Si cobrás señas online, MercadoPago '
          + 'descuenta lo suyo tengas abono o no.',
          'El abono suele estar escalonado por cantidad de canchas, así que sumar una cancha puede saltarte de '
          + 'plan sin que cambie nada más.',
          'El precio anunciado más bajo suele ser pagando el año por adelantado. Mes a mes cuesta más, y es el '
          + 'número que corresponde comparar si no vas a adelantar doce meses.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Conviene pagar el año por adelantado?',
        answer:
          `Sale más barato por mes, sí: en ATC el Plan Base pasa de ${dolares(ATC_PLANES[0].mensual)} a `
          + `${dolares(ATC_PLANES[0].anual)} pagando los doce juntos. Lo que estás comprando con ese descuento es `
          + 'quedarte un año, así que la pregunta real no es el precio sino qué pasa si a los tres meses te das '
          + 'cuenta de que no era para vos.',
      },
      {
        question: '¿Cuántas reservas por mes hace un complejo típico?',
        answer:
          'Depende demasiado de la cantidad de canchas y de la ocupación como para dar un número honesto. Lo '
          + 'que sí podés hacer es contar los turnos del último mes en tu propia planilla o cuaderno, y usar '
          + 'ese número contra la tabla de arriba. Es tu dato, no un promedio de nadie.',
      },
      {
        question: '¿El cargo por reserva no espanta a los clientes?',
        answer:
          'Es la pregunta correcta y la respuesta honesta es que no lo sabemos todavía: Vibe no abrió, así que '
          + 'no hay datos propios que mostrar. Lo que sí se puede decir es que el cargo se ve antes de pagar, no '
          + 'después, y que se reembolsa junto con la seña si la reserva se cancela en plazo.',
      },
      {
        question: '¿Hay opciones gratis de verdad?',
        answer:
          'Hay planes gratuitos con límites de canchas o de reservas, y pruebas por tiempo limitado. Antes de '
          + 'contarlos como gratis conviene mirar dos cosas: qué pasa cuando pasás el límite, y si el cobro de '
          + 'señas online está incluido o es un extra, porque suele ser lo primero que queda afuera.',
      },
    ],
  },
  {
    slug: 'como-llenar-los-horarios-vacios-de-un-complejo',
    tutorial: { slug: 'panel-de-control', texto: 'El mapa de ocupación que muestra tus horas muertas' },
    title: 'Cómo llenar los horarios vacíos de un complejo',
    seoTitle: 'Cómo llenar los horarios vacíos de tu complejo deportivo',
    /* Esta guía es el género donde se inventan estadísticas ("+30% de ocupación
       en 60 días"). No hay ninguna: Vibe no abrió y no tiene datos propios que
       mostrar, así que lo único que se afirma es el mecanismo. Los porcentajes
       que sí aparecen son los del cargo y los de MercadoPago, y salen de las
       constantes, no del teclado. */
    metaDescription:
      'Las horas muertas se llenan con diagnóstico, reserva sin teléfono y seña, no con promociones. '
      + 'Cinco pasos concretos, sin estadísticas inventadas.',
    excerpt:
      'La ocupación promedio es el número que menos te sirve. Qué mirar, en qué orden, '
      + 'y qué hacer hoy con cada franja que quedó vacía.',
    respuesta:
      'No se llenan con una promoción: se llenan sacando de encima, una por una, las cosas que impiden que '
      + 'entre una reserva. Primero mirás una semana de datos reales para saber qué horas están vacías de '
      + 'verdad y en qué cancha. Después hacés que se pueda reservar sin hablar con nadie, a la hora que sea. '
      + 'Después pedís seña, que es lo que convierte un "te aviso" en un turno. Después ponés ese link donde '
      + 'la gente ya te busca. Y por último volvés sobre los clientes que ya vinieron, que son los más baratos '
      + 'de traer. Ninguno de los cinco pasos necesita Vibe: necesitan estar hechos.',
    datePublished: '2026-09-15',
    bloques: [
      {
        tipo: 'parrafos',
        heading: 'Primero: cuáles son las horas muertas de verdad',
        parrafos: [
          '"A la mañana está vacío" es una impresión, no un dato, y casi siempre es media verdad: lo que está '
          + 'vacío es el martes a las diez, no la mañana. La diferencia no es un detalle. Una promoción para '
          + 'toda la mañana regala descuento en los turnos que se vendían igual, y deja el martes como estaba.',
          'Lo que hace falta es una semana entera, anotada hora por hora y cancha por cancha. Sirve un cuaderno. '
          + 'Lo que no sirve es el recuerdo, porque el recuerdo guarda los sábados llenos y no guarda los martes.',
          'Después hay que separar dos cosas que se mezclan siempre: los días de semana y el fin de semana. Un '
          + 'complejo puede tener el sábado casi lleno y el miércoles casi vacío, y dar un promedio decente que '
          + 'no describe ninguno de los dos. La ocupación promedio es el número que menos te sirve de todos.',
          'Si usás Vibe, el panel ya arma parte de eso: el mapa de calor cruza hora del día contra día de la '
          + 'semana y marca el pico, el calendario de reservas muestra el día cancha por cancha, y el reporte '
          + 'mensual abre las reservas y la plata por cancha. Nada de eso te dice qué hacer; te dice dónde mirar.',
          'Qué hacer hoy: anotá una semana completa, hora por hora y cancha por cancha, y marcá las franjas que '
          + 'quedaron vacías. Esas franjas son el problema, no "la mañana".',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Que se pueda reservar sin que nadie atienda',
        parrafos: [
          'Una parte de los turnos vacíos no está vacía porque nadie los quiera: está vacía porque cuando '
          + 'alguien quiso reservarlos no había con quién hablar. El mensaje entra a las once de la noche, se '
          + 'contesta a las nueve de la mañana, y a las nueve de la mañana esa persona ya jugó en otro lado.',
          'Qué proporción de reservas se pierde así no lo sabemos y no lo vamos a inventar: depende de tu '
          + 'complejo y de tu horario de atención. Lo que sí es medible, y en tu propio teléfono, es cuántos '
          + 'mensajes de reserva te entraron fuera del horario en el que contestás.',
          'El mecanismo no tiene vuelta. Si hay una página donde se ve la grilla libre y se reserva sin esperar '
          + 'respuesta, la reserva entra a la hora que entra y el turno deja de depender de que vos estés '
          + 'despierto. El trabajo real no es el software: es que la grilla esté cargada de verdad, con los '
          + 'horarios y los precios al día. Una página que muestra libre un horario que no lo está hace más '
          + 'daño que no tener página.',
          'Qué hacer hoy: contá en tu WhatsApp los mensajes de reserva que entraron fuera de tu horario de '
          + 'atención esta semana. Ese número es tuyo, es real, y es el tamaño de lo que estás perdiendo.',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'La seña es lo que separa un turno de un "te aviso"',
        parrafos: [
          'Reservar sin pagar nada no cuesta nada, y lo que no cuesta nada se cancela sin avisar. El que puso '
          + 'plata se presenta. Esa es toda la función de la seña: no es financiamiento, es un filtro.',
          'Y filtra en los dos sentidos. Al que iba a ir no lo espanta, porque ya pensaba pagar. Al que estaba '
          + 'tanteando lo saca de la grilla ahora, que es cuando todavía podés vender ese turno, en vez de a '
          + 'las ocho de la noche, cuando ya no se lo vendés a nadie.',
          `En Vibe el complejo no paga abono. Al cliente que reserva se le suma un cargo de servicio del `
          + `${CARGO_SERVICIO_TEXTO} sobre la seña, con un mínimo de ${pesos(CARGO_MINIMO)}: sobre una seña de `
          + `${pesos(SENA_DE_EJEMPLO)} son ${pesos(cargoPorReserva())}, y los paga él, no vos. Se ve antes de `
          + `pagar, no después. Si la reserva se cancela dentro de la ventana de cancelación que configuró el `
          + `complejo, la seña se reembolsa automáticamente por MercadoPago y el cargo de servicio se devuelve `
          + `junto con ella.`,
          `Aparte está la comisión de MercadoPago, que esa sí la paga el complejo: se descuenta de la seña antes `
          + `de que el dinero llegue a tu cuenta, y va ${RANGO_NACIONAL} más IVA según tu provincia y el plazo `
          + `de acreditación que elijas. La tabla completa está en `
          + '<a href="/guias/cuanto-cobra-mercadopago-por-una-sena">cuánto cobra MercadoPago por una seña</a>.',
          'Qué hacer hoy: definí un porcentaje de seña y una ventana de cancelación, y escribilos en el mismo '
          + 'lugar donde la gente reserva. Una regla escrita se discute mucho menos que una regla que hay que '
          + 'explicar por teléfono cada vez.',
        ],
      },
      {
        tipo: 'lista',
        heading: 'Dónde te tienen que encontrar',
        intro:
          'El que busca cancha un viernes a la tarde no entra a tu web: busca en Google, mira Instagram o '
          + 'manda un WhatsApp. En esos tres lugares tiene que estar el mismo link, y tiene que llevar a la '
          + 'grilla donde se reserva, no a una portada.',
        items: [
          'La ficha de Google de tu complejo. Es lo que aparece cuando alguien busca canchas más el nombre del '
          + 'barrio, y suele estar cargada a medias: sin horarios, sin fotos y con el teléfono como único '
          + 'contacto. El campo del sitio web tiene que apuntar a donde se reserva.',
          'La bio de Instagram. Un link, el de reservar. Si hay cinco, el que importa se pierde; y si el único '
          + 'llamado a la acción es "mandanos un DM", volviste a depender de que alguien conteste.',
          'El WhatsApp del complejo. El mensaje automático de bienvenida es el lugar más barato que existe para '
          + 'poner el link: contesta solo, a cualquier hora, y le contesta a alguien que ya te está escribiendo.',
          'El mismo link en los tres. Si Google, Instagram y WhatsApp llevan a tres lugares distintos, vos no '
          + 'sabés cuál funciona y el que reserva no sabe cuál es el oficial.',
          'Qué hacer hoy: abrí los tres y fijate si llevan al mismo lado. El que no tenga link es, hoy, el que '
          + 'te está mandando la gente al teléfono.',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'Los que ya vinieron son los más baratos de traer',
        parrafos: [
          'Conseguir un cliente nuevo cuesta plata o cuesta tiempo. Volver a traer a uno que ya jugó en tu '
          + 'cancha, que sabe dónde queda y cuánto sale, cuesta un mensaje.',
          'La lista ya la tenés, aunque esté repartida entre el cuaderno y el historial de WhatsApp. Lo que '
          + 'falta es ordenarla en tres montones: el que viene todas las semanas, el que vino varias veces y '
          + 'hace rato que no aparece, y el que reservó y no se presentó. Son tres conversaciones distintas y '
          + 'merecen tres mensajes distintos.',
          'Al habitué no hay que venderle nada: hay que ofrecerle el turno fijo. Al que dejó de venir se le '
          + 'escribe una vez, sin promoción, preguntando si pasó algo, y la respuesta suele explicar algo de '
          + 'tu complejo que no sabías. Al que no se presentó se le pide seña la próxima vez, y listo.',
          'En Vibe la ficha de cada cliente guarda sus reservas, sus ausencias y su asistencia, y el panel '
          + 'separa a los nuevos de los recurrentes y muestra los que más vienen. Sin Vibe, la misma '
          + 'información está en tu cuaderno: da más trabajo sacarla, no es imposible.',
          'Si estás evaluando con qué herramienta hacer todo esto, los dos modelos de precio que hay en '
          + 'Argentina están abiertos con números en '
          + '<a href="/guias/cuanto-cuesta-un-sistema-de-reservas-para-canchas">cuánto cuesta un sistema de '
          + 'reservas para canchas</a>.',
          'Qué hacer hoy: sacá de tu historial los clientes que venían seguido y hace rato que no aparecen, y '
          + 'escribiles uno por uno. Sin lista de difusión: un mensaje reenviado se nota y no se contesta.',
        ],
      },
    ],
    faq: [
      {
        question: '¿Cuánto tiempo hay que medir antes de tocar algo?',
        answer:
          'Una semana entera y de corrido, como mínimo, porque necesitás que entren los siete días. Si podés '
          + 'anotar un mes, mejor: recién ahí se ve si el martes flojo es todos los martes o fue ese martes. '
          + 'Lo que no sirve es medir tres días buenos y decidir con eso.',
      },
      {
        question: '¿Pedir seña no me hace perder reservas?',
        answer:
          'Pierde las que se iban a caer igual, que es distinto. No tenemos datos propios para ponerle un '
          + 'número: Vibe todavía no abrió. Lo que sí se puede decir es cuándo se entera uno de que el turno '
          + 'se cayó: con seña, al momento de reservar; sin seña, cuando el turno ya pasó.',
      },
      {
        question: '¿Esto sirve si no uso ningún sistema de reservas?',
        answer:
          'Sí. Los cinco pasos son de gestión, no de software: medir una semana, tener una forma de reservar '
          + 'que no dependa de que alguien conteste, pedir seña, poner el mismo link en los tres lugares donde '
          + 'te buscan, y volver sobre los clientes que ya vinieron. Un sistema los hace más rápido y te evita '
          + 'transcribir; ninguno de los cinco lo inventa.',
      },
      {
        question: '¿Por dónde empiezo si tengo una sola cancha?',
        answer:
          'Por el segundo paso. Con una cancha el diagnóstico lo tenés en la cabeza y no te vas a equivocar '
          + 'demasiado, así que lo que más rinde es que se pueda reservar sin que atiendas: con una sola '
          + 'cancha, cada turno que se pierde por un mensaje sin contestar es un porcentaje grande de tu día.',
      },
      {
        question: '¿Cómo sé si lo que hice funcionó?',
        answer:
          'Comparando las mismas franjas contra la semana que anotaste antes de tocar nada, no la ocupación '
          + 'general. Si moviste el martes a la mañana, mirá el martes a la mañana. El promedio del complejo '
          + 'se mueve por el clima, por un feriado y por el mes del año, así que sirve para todo menos para '
          + 'saber si tu cambio hizo algo.',
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

/** Fecha de la guía más nueva, para el índice. */
export const ULTIMA_ACTUALIZACION = guias
  .map(g => g.dateModified)
  .sort()
  .at(-1) ?? VIGENTE_DESDE;
