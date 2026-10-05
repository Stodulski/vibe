/**
 * El modelo de precio de Vibe, en un solo lugar.
 *
 * Los costos de MercadoPago viven aparte (mercadopago-costos.ts) porque son de
 * un tercero y los vigila un cron. Esto es lo nuestro: el cargo de servicio y
 * las cuentas que la página muestra como ejemplo.
 *
 * Existe por lo mismo que el otro archivo. El monto del cargo estaba escrito a
 * mano en la sección de precio, en la FAQ visible, en el FAQPage del schema y en
 * los términos. Cuatro copias de una afirmación sobre plata es una que alguien
 * va a actualizar sin actualizar las otras tres.
 *
 * Los MONTOS DEL EJEMPLO no se escriben: se calculan. Estaban tipeados, y son
 * derivados de una tasa, así que basta que MercadoPago mueva una tasa para que
 * un número tipeado quede contradiciendo a la tabla de arriba en la misma
 * página.
 */
import { GRUPO_DE_REFERENCIA, PLAZOS } from './mercadopago-costos.ts';

/**
 * El cargo de servicio, en pesos: un monto fijo por pago online, sin importar la
 * seña. Se le suma al cliente que reserva, nunca al complejo.
 *
 * Repite backend/internal/pricing/pricing.go, que es lo que se cobra de verdad.
 * Cambiarlo solo hace fallar CI: .github/scripts/check-service-fee.mjs compara
 * esta copia contra las otras.
 */
export const CARGO_SERVICIO = 1000;

/**
 * El aviso que acompaña a cada lugar donde se dice cuánto es el cargo. Está acá
 * para que todas las menciones digan lo mismo.
 */
export const CARGO_PUEDE_CAMBIAR =
  'El monto puede cambiar en cualquier momento, y el vigente se muestra siempre antes de pagar.';

/** La seña con la que la página hace la cuenta. Un número redondo miente menos. */
export const SENA_DE_EJEMPLO = 17400;

/** 1218 -> "$1.218". */
export const pesos = (n: number) => `$${Math.round(n).toLocaleString('es-AR')}`;

/**
 * 50 -> "50 USD". 1.25 -> "1,25 USD".
 *
 * Sin redondear a entero: a diferencia de `pesos`, esto formatea precios de
 * competencia (competencia.ts) que ya vienen en dólares, incluidos cocientes
 * como "por reserva" que dan centavos. Redondear ahí perdería la diferencia
 * entre volúmenes que es justo lo que esas cuentas quieren mostrar.
 */
export const dolares = (n: number) => `${Number(n.toFixed(2)).toLocaleString('es-AR')} USD`;

/**
 * La cuenta completa del ejemplo, derivada de las dos fuentes.
 *
 * Se usa el grupo de referencia de MercadoPago (Buenos Aires) porque es donde
 * está la mayor parte del volumen, y sus dos extremos: la acreditación inmediata
 * y la más lenta, que son las que marcan la banda.
 */
export function ejemplo(sena: number = SENA_DE_EJEMPLO) {
  const cargo = CARGO_SERVICIO;
  const tasas = GRUPO_DE_REFERENCIA.tasas;
  const inmediata = Math.round(sena * tasas[0] / 100);
  const masLenta = Math.round(sena * tasas[tasas.length - 1] / 100);

  return {
    sena,
    cargo,
    /** Lo que efectivamente sale del bolsillo del cliente. */
    totalCliente: sena + cargo,
    /** Lo que MercadoPago descuenta en cada extremo, y lo que queda. */
    inmediata: { plazo: PLAZOS[0], descuento: inmediata, neto: sena - inmediata },
    masLenta: {
      plazo: PLAZOS[PLAZOS.length - 1],
      descuento: masLenta,
      neto: sena - masLenta,
    },
  };
}
