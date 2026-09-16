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
export const ATC_VERIFICADO = '2026-09-16';

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
