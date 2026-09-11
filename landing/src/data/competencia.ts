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
 * Si se vuelven a mostrar, hay que mover ATC_VERIFICADO en el mismo commit. Un
 * precio de la competencia con fecha vieja es peor que ninguno.
 */

/** De dónde salen los precios de abajo. */
export const ATC_FUENTE = 'https://atcsports.io/software-gestion-deportiva';

/** Cuándo se leyeron por última vez de esa página. */
export const ATC_VERIFICADO = '2026-08-24';

/**
 * Los planes que publica ATC Sports, en pesos por mes.
 *
 * `mensual` es lo que cobran pagando mes a mes. `anual` es el precio por mes
 * pagando los doce por adelantado, que es el que usan para el "desde $57.000"
 * del encabezado. Comparar contra el de ellos sin aclarar cuál es sería elegir
 * el número que más conviene.
 */
export const ATC_PLANES = [
  { nombre: 'Base', canchas: '1 a 3', mensual: 71000, anual: 57000 },
  { nombre: 'Estándar', canchas: '4 a 6', mensual: 111000, anual: 89000 },
  { nombre: 'Full', canchas: '7 o más', mensual: 145000, anual: 116000 },
];

export const ATC_ABONO_MINIMO = Math.min(...ATC_PLANES.map(p => p.mensual));
export const ATC_ABONO_MAXIMO = Math.max(...ATC_PLANES.map(p => p.mensual));
