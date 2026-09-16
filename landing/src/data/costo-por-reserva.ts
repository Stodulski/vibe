/**
 * Cuánto sale cada reserva con un abono fijo y cuánto con un cargo por reserva.
 *
 * Es la única cuenta que contesta "cuánto cuesta un sistema de reservas", porque
 * el precio de lista no dice nada sin el volumen: el mismo abono es carísimo o
 * baratísimo según cuántas reservas le pongas encima.
 *
 * NO ESTÁ ACÁ PARA QUE VIBE GANE LA COMPARACIÓN. La cuenta muestra que a volumen
 * alto el cargo por reserva extrae mucho más por turno que el abono, y eso es
 * verdad y va escrito. Una comparación en la que uno siempre gana no la lee
 * nadie dos veces, y con razón.
 *
 * Todo sale de datos que ya vigilamos: los planes de ATC (competencia.ts, con
 * fuente y fecha) y nuestro propio cargo (precio.ts). Ninguno se escribe acá.
 */
import { ATC_PLANES } from './competencia.ts';
import { CARGO_SERVICIO, CARGO_MINIMO, SENA_DE_EJEMPLO, dolares } from './precio.ts';

/** Los volúmenes de la tabla. Un complejo chico y uno grande, y el medio. */
export const VOLUMENES = [50, 100, 200, 400, 800];

/** Lo que sale cada reserva si pagás un abono fijo de `mensual` y hacés `n`. */
export const porReservaConAbono = (mensual: number, n: number) => mensual / n;

/** Lo que se le suma al cliente por reserva, con el mínimo aplicado. */
export const cargoPorReserva = (sena: number = SENA_DE_EJEMPLO) =>
  Math.max(CARGO_MINIMO, Math.round(sena * CARGO_SERVICIO));

/** La tabla: una fila por volumen, una columna por plan. */
export const filasDeCostoPorReserva = VOLUMENES.map(n => [
  `${n} reservas`,
  ...ATC_PLANES.map(p => dolares(porReservaConAbono(p.mensual, n))),
]);

export const columnasDeCostoPorReserva = [
  'Reservas por mes',
  ...ATC_PLANES.map(p => `Plan ${p.nombre}`),
];

/* Los dos extremos, para poder decirlos en el texto sin tipearlos. */
const barato = ATC_PLANES[0].mensual;
export const POR_RESERVA_POCAS = porReservaConAbono(barato, VOLUMENES[0]);
export const POR_RESERVA_MUCHAS = porReservaConAbono(barato, VOLUMENES.at(-1)!);

/** Cuántas veces más caro sale cada reserva en el volumen más bajo. */
export const VECES = Math.round(POR_RESERVA_POCAS / POR_RESERVA_MUCHAS);
