/**
 * Lo que se sabe de la competencia, con fuente.
 *
 * Existe porque la tabla comparativa afirmaba "Hasta $145.000 por mes fijo"
 * escrito dentro del componente, sin fuente y sin fecha. Afirmar plata ajena
 * sobre un competidor con nombre es de las cosas más caras que puede tener una
 * landing, y ese número no se podía respaldar.
 *
 * Se fue a verificar y RESULTÓ SER CIERTO: es el precio mensual del plan más
 * caro que ATC Sports publica en su propia página. Igual la tabla ya no lo
 * muestra, por decisión de producto: "abono mensual fijo" es lo que hace la
 * diferencia con Vibe, se sostiene solo, y no envejece cada vez que ellos
 * ajustan la lista. Los números quedan acá por si se los quiere volver a usar.
 *
 * Los precios se guardan en DÓLARES, no en pesos, aunque ATC vende en
 * Argentina. La página de ATC está geolocalizada: a una visita desde
 * Argentina le muestra pesos, y a cualquier otra le muestra dólares. Eso solo
 * tiene sentido si el dólar es el precio de lista y el peso es una conversión
 * que ellos recalculan solos puertas adentro. Guardar la versión en pesos
 * significa perseguir el tipo de cambio para siempre —el mismo problema que
 * este archivo existe para evitar, solo que con un paso intermedio en vez de
 * con inflación directa—. El dólar es el número que no se mueve solo.
 *
 * Si se vuelven a mostrar, hay que mover ATC_VERIFICADO en el mismo commit. Un
 * precio de la competencia con fecha vieja es peor que ninguno.
 */

/** De dónde salen los precios de abajo. */
export const ATC_FUENTE = 'https://atcsports.io/software-gestion-deportiva';

/** Cuándo se leyeron por última vez de esa página. */
export const ATC_VERIFICADO = '2026-09-23';

/**
 * Los planes que publica ATC Sports, en dólares por mes.
 *
 * `mensual` es lo que cobran pagando mes a mes. `anual` es el precio por mes
 * pagando los doce por adelantado. Comparar contra el de ellos sin aclarar
 * cuál es sería elegir el número que más conviene.
 */
export const ATC_PLANES = [
  { nombre: 'Base', canchas: '1 a 3', mensual: 50, anual: 40 },
  { nombre: 'Estándar', canchas: '4 a 6', mensual: 80, anual: 64 },
  { nombre: 'Full', canchas: '7 o más', mensual: 100, anual: 80 },
];

export const ATC_ABONO_MINIMO = Math.min(...ATC_PLANES.map(p => p.mensual));
export const ATC_ABONO_MAXIMO = Math.max(...ATC_PLANES.map(p => p.mensual));

/**
 * Lo demás que ATC publica junto a la lista de precios, verificado el mismo
 * día que `ATC_PLANES`. No es precio, pero cambia la comparación: treinta
 * días de prueba sin cargo bajan el costo de probar, y un abono que ya
 * incluye puesta en marcha y soporte no tiene costos escondidos que sumarle
 * al `mensual` de arriba.
 */
export const ATC_PRUEBA_GRATIS_DIAS = 30;
export const ATC_SIN_COSTO_DE_CONFIGURACION = true;
export const ATC_SIN_COSTO_DE_CAPACITACION = true;
export const ATC_SOPORTE_REMOTO_SIN_COSTO_ADICIONAL = true;

/**
 * Así lo publican ellos junto a la tabla: "33% de descuento pagando anual".
 * No hace falta que cuadre con la resta entre `mensual` y `anual` de arriba
 * —puede que redondeen distinto—, y no se recalcula acá: se guarda tal cual
 * lo dicen.
 */
export const ATC_DESCUENTO_ANUAL_PORCENTAJE = 33;

/** No mencionan comisión por reserva en ninguna parte de esa página. */
export const ATC_MENCIONA_COMISION_POR_RESERVA = false;

/* ─────────────────────────── CanchaFija ─────────────────────────── */

/**
 * CanchaFija cotiza en pesos, y ahí los pesos SÍ son la unidad real: es el
 * precio que ellos fijan, no una conversión. Por eso acá se guardan en pesos y
 * no en dólares como los de ATC.
 *
 * Lo que no se arregla con la moneda es la inflación: una lista en pesos se
 * actualiza sola del lado de ellos y en silencio del nuestro. `VERIFICADO` es
 * lo único que avisa. Si pasaron meses, hay que releer antes de publicar.
 */
export const CF_FUENTE = 'https://canchafija.com.ar/precios';
export const CF_FUENTE_TERMINOS = 'https://canchafija.com.ar/terminos';
export const CF_VERIFICADO = '2026-09-23';

/** Planes publicados, en pesos por mes. */
export const CF_PLANES = [
  { nombre: 'Lite', canchas: 1, mensual: 10000 },
  { nombre: 'Inicial', canchas: 3, mensual: 18000 },
  { nombre: 'Pro', canchas: 6, mensual: 25000 },
  { nombre: 'Club', canchas: 10, mensual: 50000 },
  { nombre: 'Premium', canchas: 15, mensual: 60000 },
];

export const CF_PRIMER_MES_GRATIS = true;
export const CF_SIN_COSTO_DE_ALTA = true;
export const CF_SIN_PERMANENCIA = true;

/**
 * Cómo cobran del lado del jugador, citado textual de sus términos.
 *
 * Es el mismo modelo que Vibe —lo paga el que reserva, no el complejo— con una
 * diferencia que vale un párrafo: ellos lo meten adentro del precio mostrado y
 * nosotros lo mostramos aparte. Ninguna de las dos formas es la correcta, y por
 * eso la comparación lo cuenta en vez de puntuarlo.
 */
export const CF_FEE_AL_JUGADOR =
  'Todos los precios mostrados en la plataforma incluyen el fee de uso de la plataforma';

/**
 * No publican el porcentaje del fee. Se buscó en precios y en términos y no
 * está, y tampoco se pudo inferir mirando sus páginas públicas de complejos,
 * porque ninguna muestra el precio de un turno. Afirmar un número acá sería
 * inventarlo.
 */
export const CF_PORCENTAJE_DEL_FEE_PUBLICADO = false;

/**
 * El alta del complejo no es self-service: se completa un formulario y ellos
 * mandan las credenciales. El jugador sí se registra solo.
 */
export const CF_ALTA_ASISTIDA = true;
/** Lo que ellos dicen que tarda configurar el complejo una vez que entrás. */
export const CF_MINUTOS_DE_CONFIGURACION = 15;
/** Mismo soporte en los cinco planes, en días hábiles. */
export const CF_SOPORTE_EN_TODOS_LOS_PLANES = true;
export const CF_SOPORTE_HORARIO = 'lunes a viernes de 9 a 18';
/** Viene incluida en todos los planes, con tope de productos. No es un extra pago. */
export const CF_TIENDA_INCLUIDA = true;
export const CF_TIENDA_TOPE_DE_PRODUCTOS = 50;
/** La política de cancelación la fija cada complejo, no la plataforma. */
export const CF_CANCELACION_LA_FIJA_EL_COMPLEJO = true;
export const CF_REEMBOLSO_DIAS_HABILES = 10;

/* ──────────────────────────── Turnito ───────────────────────────── */

/**
 * Turnito no es software de canchas: es una agenda de turnos para peluquerías,
 * consultorios y gimnasios, que también apunta a clubes. Maneja "agendas", no
 * canchas, y eso es lo que la comparación tiene que explicar antes que el
 * precio.
 *
 * Los montos son de la página de Argentina. Cada país tiene la suya y México
 * cotiza en dólares, así que un precio de acá no vale para allá.
 */
export const TU_FUENTE = 'https://turnito.app/ar/planes/';
export const TU_VERIFICADO = '2026-09-16';

/** Planes de Argentina. `mensual` es con IVA, que es lo que termina pagando. */
export const TU_PLANES = [
  { nombre: 'Gratuito', mensual: 0, comision: 5, reservas: '100 por mes' },
  { nombre: 'Plus', mensual: 12000, comision: 3.5, reservas: '200 por mes' },
  { nombre: 'Advance', mensual: 24500, comision: 1, reservas: 'sin límite' },
  { nombre: 'Pro', mensual: 42000, comision: 0, reservas: 'sin límite' },
];

export const TU_PLAN_GRATIS_NO_VENCE = true;
export const TU_SIN_PERMANENCIA = true;
/** La comisión corre solo sobre lo que se cobra online por Turnito, no sobre efectivo. */
export const TU_COMISION_SOLO_ONLINE = true;
/**
 * La comisión la absorbe el negocio, no el cliente.
 *
 * No hay una frase que lo diga, así que sale de dos hechos: "El dinero va
 * directo a tu cuenta, no pasa por Turnito", y que en una reserva real el que
 * paga ve un total sin ninguna línea de comisión. Es el modelo inverso al
 * nuestro, donde el cargo lo paga quien reserva.
 */
export const TU_LA_COMISION_LA_ABSORBE_EL_NEGOCIO = true;

/**
 * Cuántas agendas permite cada plan, en el orden de TU_PLANES.
 *
 * Importa más que el precio. En su producto un enlace de reserva es UNA agenda:
 * elegís fecha y te aparece una lista plana de horarios, sin nada que nombre
 * una cancha. Si cada cancha es una agenda —y todo indica que sí, aunque ellos
 * no lo escriban con esas palabras— un complejo de seis canchas no entra en los
 * dos planes más baratos.
 */
export const TU_AGENDAS_POR_PLAN = [3, 5, Infinity, Infinity];
/** Verificado entrando a tres páginas de reserva reales, una de ellas un pádel. */
export const TU_SIN_GRILLA_DE_CANCHAS = true;
/** Recurrentes activas por plan; en los dos primeros no figuran. */
export const TU_RECURRENTES_POR_PLAN = [0, 0, 15, Infinity];
/** Qué pasa al pasarse del tope de reservas no está publicado en ningún lado. */
export const TU_QUE_PASA_AL_PASARSE_DEL_TOPE_PUBLICADO = false;
