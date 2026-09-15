/**
 * El modelo de precio de Vibe, en un solo lugar.
 *
 * Los costos de MercadoPago viven aparte (mercadopago-costos.ts) porque son de
 * un tercero y los vigila un cron. Esto es lo nuestro: el cargo de servicio, su
 * mínimo, y las cuentas que la página muestra como ejemplo.
 *
 * Existe por lo mismo que el otro archivo. El 7% y el mínimo estaban escritos a
 * mano en la sección de precio, en la FAQ visible, en el FAQPage del schema y en
 * los términos. Cuatro copias de una afirmación sobre plata es una que alguien
 * va a actualizar sin actualizar las otras tres.
 *
 * Los MONTOS DEL EJEMPLO no se escriben: se calculan. Estaban tipeados, y son
 * derivados de un porcentaje, así que basta que MercadoPago mueva una tasa para
 * que un número tipeado quede contradiciendo a la tabla de arriba en la misma
 * página.
 */
import { GRUPO_DE_REFERENCIA, PLAZOS } from './mercadopago-costos.ts';

/**
 * Se le suma al cliente que reserva, sobre la seña, nunca sobre el total.
 *
 * Repite backend/internal/pricing/pricing.go, que es lo que se cobra de verdad.
 * Cambiarlo solo hace fallar CI: .github/scripts/check-service-fee.mjs compara
 * esta copia contra las otras tres.
 */
export const CARGO_SERVICIO = 0.07;

/** Piso del cargo, en pesos. */
export const CARGO_MINIMO = 1000;

/** La seña con la que la página hace la cuenta. Un número redondo miente menos. */
export const SENA_DE_EJEMPLO = 17400;

/** "7%", como se escribe en el texto. */
/* Redondeado antes de imprimir: 0.07 * 100 da 7.000000000000001 en coma flotante. */
export const CARGO_SERVICIO_TEXTO = `${Number((CARGO_SERVICIO * 100).toFixed(2)).toString().replace('.', ',')}%`;

/** 1218 -> "$1.218". */
export const pesos = (n: number) => `$${Math.round(n).toLocaleString('es-AR')}`;

/** Lo que se le suma al cliente por una seña dada, con el mínimo aplicado. */
export const cargoDeServicio = (sena: number) =>
  Math.max(CARGO_MINIMO, Math.round(sena * CARGO_SERVICIO));

/**
 * La cuenta completa del ejemplo, derivada de las dos fuentes.
 *
 * Se usa el grupo de referencia de MercadoPago (Buenos Aires) porque es donde
 * está la mayor parte del volumen, y sus dos extremos: la acreditación inmediata
 * y la más lenta, que son las que marcan la banda.
 */
export function ejemplo(sena: number = SENA_DE_EJEMPLO) {
  const cargo = cargoDeServicio(sena);
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
