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
} from './mercadopago-costos.ts';
import { ejemplo, pesos } from './precio.ts';
import { fechaDeContenido, guardarFechas } from './fecha-de-contenido.ts';
import { ATC_PLANES, ATC_FUENTE, ATC_VERIFICADO } from './competencia.ts';
import {
  filasDeCostoPorReserva, columnasDeCostoPorReserva, cargoPorReserva,
  porReservaConAbono, POR_RESERVA_POCAS, POR_RESERVA_MUCHAS, VECES, VOLUMENES,
} from './costo-por-reserva.ts';
import { CARGO_SERVICIO_TEXTO, SENA_DE_EJEMPLO } from './precio.ts';

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
  fuente?: { nombre: string; url: string };
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
    slug: 'cuanto-cobra-mercadopago-por-una-sena',
    title: 'Cuánto cobra MercadoPago por cobrar una seña',
    seoTitle: 'Cuánto cobra MercadoPago por una seña | Tabla por provincia',
    /* Medida en pixeles, no en caracteres: 772px sobre un limite de 920. El margen
       importa porque el texto se arma desde el dataset, asi que si MercadoPago
       mueve una tasa cambia el largo. */
    metaDescription:
      `MercadoPago cobra de ${porcentaje(MINIMO)} a ${porcentaje(MAXIMO)} más IVA por una seña. `
      + 'La tabla completa por provincia y plazo, con su fuente oficial.',
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
          + 'número que te cierra en la cuenta nunca coincide con el porcentaje que leíste.',
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
    title: 'Cuánto cuesta un sistema de reservas para canchas',
    seoTitle: 'Cuánto cuesta un sistema de reservas de canchas | Argentina',
    metaDescription:
      `Abono fijo desde ${pesos(ATC_PLANES[0].mensual)} por mes, o un cargo por reserva. `
      + 'Cuál conviene según cuántos turnos hacés, con la cuenta hecha.',
    excerpt:
      'El precio de lista no dice nada sin el volumen. La misma cuota sale '
      + `${pesos(POR_RESERVA_POCAS)} o ${pesos(POR_RESERVA_MUCHAS)} por reserva según cuántas hagas.`,
    respuesta:
      `En Argentina hay dos modelos. Un abono mensual fijo, que arranca en ${pesos(ATC_PLANES[0].mensual)} `
      + `y llega a ${pesos(ATC_PLANES.at(-1)!.mensual)} según cuántas canchas tengas, y se paga haya reservas o no. `
      + `O un cargo por reserva, que no cobra nada fijo. Cuál conviene depende de una sola cosa: cuántos `
      + `turnos hacés por mes. El mismo abono de ${pesos(ATC_PLANES[0].mensual)} sale ${pesos(POR_RESERVA_POCAS)} `
      + `por reserva si hacés ${VOLUMENES[0]} al mes, y ${pesos(POR_RESERVA_MUCHAS)} si hacés ${VOLUMENES.at(-1)}.`,
    datePublished: '2026-08-24',
    fuente: { nombre: 'ATC Sports, precios y planes', url: ATC_FUENTE },
    bloques: [
      {
        tipo: 'tabla',
        heading: 'Lo que sale cada reserva con un abono fijo',
        intro:
          'La cuota dividida por la cantidad de turnos del mes. Es la cuenta que no aparece en '
          + 'ninguna página de precios, y es la única que te dice si te conviene.',
        nota:
          `Precios de lista de ATC Sports, pagando mes a mes, verificados el ${ATC_VERIFICADO}. `
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
          + 'Un enero flojo o dos semanas de lluvia no bajan la cuota.',
          'La contracara, que es real y conviene decirla: el abono es previsible. Sabés exactamente cuánto vas '
          + 'a pagar el mes que viene, y para presupuestar eso vale. Un cargo por reserva sube cuando te va bien.',
        ],
      },
      {
        tipo: 'parrafos',
        heading: 'El otro modelo, y quién paga qué',
        parrafos: [
          `Vibe no cobra abono: el complejo paga $0 fijo. Lo que hay es un cargo de servicio del `
          + `${CARGO_SERVICIO_TEXTO} sobre la seña, que se le suma al cliente que reserva. Sobre una seña de `
          + `${pesos(SENA_DE_EJEMPLO)} son ${pesos(cargoPorReserva())}, y los paga él, no el complejo.`,
          'Para comparar los dos modelos sin marearse hay que separar los bolsillos, porque no es el mismo el '
          + 'que paga cada cosa.',
          `Del lado del complejo: con los dos modelos se paga la comisión de MercadoPago, y en los dos casos es `
          + `sobre la seña, no sobre el precio total de la cancha. Esa parte se cancela. Lo que queda de `
          + `diferencia es exactamente el abono: a ${VOLUMENES[2]} reservas por mes, `
          + `${pesos(porReservaConAbono(ATC_PLANES[0].mensual, VOLUMENES[2]))} por reserva de más con el plan más barato.`,
          `Del lado del cliente es al revés: con un abono paga el precio de la cancha y nada más, y con un cargo `
          + `por reserva paga el precio más ${pesos(cargoPorReserva())}. Ahí el abono le sale más barato a él.`,
          'Resumido sin vueltas: el modelo de cargo por reserva le saca el costo fijo al complejo y se lo pasa a '
          + 'quien reserva. Si estás del lado del mostrador te conviene siempre; si estás del otro, depende de '
          + 'cuánto valga para vos reservar y pagar desde el celular en vez de por teléfono.',
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
          `Sale más barato por mes, sí: en ATC el Plan Base pasa de ${pesos(ATC_PLANES[0].mensual)} a `
          + `${pesos(ATC_PLANES[0].anual)} pagando los doce juntos. Lo que estás comprando con ese descuento es `
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
