/**
 * Costos de Checkout de MercadoPago, copiados de la fuente primaria.
 *
 * Fuente: https://www.mercadopago.com.ar/ayuda/33399
 * ("¿Cuánto cuesta recibir pagos con Checkout?")
 *
 * Vibe cobra las señas con Checkout, así que esta es la tabla que aplica. La
 * página general de costos (/ayuda/costo-recibir-pagos_220) no publica ningún
 * porcentaje: solo deriva a la de cada producto.
 *
 * Tres cosas que esta tabla deja claras y que conviene no perder de vista:
 *
 * 1. El costo NO es nacional. Depende de la provincia donde está registrado el
 *    domicilio del que cobra, y hay nueve grupos distintos. Entre el más caro y
 *    el más barato hay 0,41 puntos en la acreditación inmediata.
 * 2. Los porcentajes NO incluyen IVA ni retenciones. Lo que termina saliendo es
 *    más que el número de la tabla, así que presentarlo como el costo final
 *    sería mentir por omisión.
 * 3. Tienen fecha de vigencia. MercadoPago los cambia, y un número sin fecha
 *    envejece sin que nadie se entere.
 *
 * Si esto se actualiza, hay que mover VIGENTE_DESDE en el mismo commit.
 */

/** Fecha desde la que MercadoPago declara vigentes estos costos. */
export const VIGENTE_DESDE = '2026-03-06';

/** La página que publica la tabla, para citarla donde se usen estos números. */
export const FUENTE = 'https://www.mercadopago.com.ar/ayuda/33399';

/** Los plazos de acreditación, en el orden en que MercadoPago los lista. */
export const PLAZOS = ['Al instante', '10 días', '18 días', '35 días'] as const;

export type GrupoDeCostos = {
  /** Provincias que comparten exactamente la misma tabla. */
  provincias: string[];
  /** Un porcentaje por plazo, alineado con PLAZOS. */
  tasas: [number, number, number, number];
};

export const COSTOS: GrupoDeCostos[] = [
  { provincias: ['Buenos Aires', 'Chubut', 'Entre Ríos'], tasas: [6.60, 4.61, 3.56, 1.56] },
  { provincias: ['Córdoba'], tasas: [6.60, 4.61, 3.56, 1.56] },
  { provincias: ['La Rioja'], tasas: [6.49, 4.53, 3.50, 1.54] },
  { provincias: ['Catamarca', 'Formosa', 'Mendoza'], tasas: [6.46, 4.51, 3.48, 1.53] },
  { provincias: ['Santa Fe'], tasas: [6.42, 4.48, 3.46, 1.52] },
  { provincias: ['Río Negro'], tasas: [6.39, 4.46, 3.44, 1.51] },
  {
    provincias: [
      'Ciudad de Buenos Aires', 'Corrientes', 'La Pampa', 'Misiones',
      'Neuquén', 'Salta', 'San Luis', 'Tierra del Fuego', 'Tucumán',
    ],
    tasas: [6.29, 4.39, 3.39, 1.49],
  },
  { provincias: ['Jujuy'], tasas: [6.29, 4.39, 3.39, 1.49] },
  {
    provincias: ['Chaco', 'San Juan', 'Santa Cruz', 'Santiago del Estero'],
    tasas: [6.19, 4.32, 3.34, 1.47],
  },
];

/** "6.6" -> "6,60%", que es como lo escribe MercadoPago y como se lee acá. */
export const porcentaje = (n: number) => `${n.toFixed(2).replace('.', ',')}%`;

const todas = COSTOS.flatMap(g => g.tasas);

/** El costo más bajo de todo el país: el plazo más largo, en el grupo más barato. */
export const MINIMO = Math.min(...todas);

/** El más alto: acreditación inmediata en el grupo más caro. */
export const MAXIMO = Math.max(...todas);

/** El grupo que aplica a la mayor parte del volumen, para los ejemplos. */
export const GRUPO_DE_REFERENCIA = COSTOS[0];

/** "entre 1,47% y 6,60%", la banda nacional tal como se dice en el texto. */
export const RANGO_NACIONAL = `entre ${porcentaje(MINIMO)} y ${porcentaje(MAXIMO)}`;

/** La tasa de un grupo, buscándolo por cualquiera de sus provincias. */
export function tasaDe(provincia: string, plazo: (typeof PLAZOS)[number]): number {
  const grupo = COSTOS.find(g => g.provincias.includes(provincia));
  if (!grupo) throw new Error(`No hay tarifa para la provincia "${provincia}"`);
  const i = PLAZOS.indexOf(plazo);
  if (i < 0) throw new Error(`No existe el plazo "${plazo}"`);
  return grupo.tasas[i];
}

/** "2026-03-06" -> "6 de marzo de 2026", para escribirlo dentro de una oración. */
export function fechaEnTexto(iso: string): string {
  const meses = ['enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio',
    'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre'];
  const [a, m, d] = iso.split('-').map(Number);
  return `${d} de ${meses[m - 1]} de ${a}`;
}

/** Cuánto separa al grupo más caro del más barato en la acreditación inmediata. */
export const BRECHA_INMEDIATA = Math.max(...COSTOS.map(g => g.tasas[0]))
  - Math.min(...COSTOS.map(g => g.tasas[0]));

/* Los otros dos numeros que declara la misma pagina de MercadoPago. Van aca por
   lo mismo que las tasas: son de un tercero y los cambia el tercero. */

/** IVA sobre la comision. Los porcentajes de la tabla se publican sin el. */
export const IVA = 21;

/** Puntos extra con tarjetas de credito extranjeras y tarjeta Sucredito. */
export const RECARGO_TARJETA_EXTRANJERA = 3;
