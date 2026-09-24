/* JSON-LD payloads copied verbatim from the pre-Astro static HTML, with the
   modification dates wired to the last commit so they cannot go stale again. */
import { LAST_MODIFIED, PUBLISHED } from './site-dates.ts';
import type { Guia } from './guias.ts';
import type { Tutorial } from './tutoriales.ts';
import { CDN, ULTIMA_ACTUALIZACION as TUTORIALES_ULTIMA_ACTUALIZACION } from './tutoriales.ts';
import { faq } from './faq.ts';
import { RANGO_NACIONAL } from './mercadopago-costos.ts';
import { CARGO_SERVICIO_TEXTO, CARGO_MINIMO, pesos } from './precio.ts';

export const homeJsonLd: string[] = [
`   {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "name": "Vibe",
    "description": "Sistema de gestión gratis para complejos deportivos: reservas online con seña, cobros en el mostrador, caja por turno con arqueo, venta de productos con stock y reportes.",
    "url": "https://vibe.com.ar",
    "applicationCategory": "BusinessApplication",
    "operatingSystem": "Web",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "ARS",
      "availability": "https://schema.org/InStock",
      "description": "100% gratis para el complejo deportivo: sin costo fijo, sin suscripción y sin cargo sobre los cobros en el mostrador ni las ventas. Al cliente que reserva online se le suma un cargo de servicio del ${CARGO_SERVICIO_TEXTO} sobre la seña, con un mínimo de ${pesos(CARGO_MINIMO)} ARS. MercadoPago descuenta su comisión del pago online, ${RANGO_NACIONAL} más IVA, según la provincia del complejo y cuándo se acredite el dinero."
    },
    "featureList": [
      "Reservas online 24/7 con página pública del complejo",
      "Cobro de señas con MercadoPago y reembolso automático",
      "Precios por cancha, día y franja horaria",
      "Cobros en el mostrador: efectivo, transferencia, débito, crédito y QR",
      "Caja por turno con arqueo de efectivo",
      "Ingresos y egresos por categoría",
      "Venta de productos con control de stock",
      "Aviso de stock bajo",
      "Recordatorio automático 2 horas antes del turno",
      "Base de clientes con asistencia y bloqueo",
      "Dashboard con ocupación por horario",
      "Reporte mensual con exportación a Excel"
    ],
    "author": {
      "@type": "Organization",
      "name": "Vibe",
      "url": "https://vibe.com.ar"
    },
    "inLanguage": "es-AR",
    "areaServed": {
      "@type": "Country",
      "name": "Argentina"
    },
    "audience": {
      "@type": "BusinessAudience",
      "name": "Complejos deportivos de pádel, tenis, fútbol, básquet, vóley, hockey y pickleball"
    },
    "datePublished": "${PUBLISHED}",
    "dateModified": "${LAST_MODIFIED}"
  }`,
`   {"@context":"https://schema.org","@type":"HowTo","name":"Cómo ordenar tu complejo deportivo con Vibe","description":"De cero a ordenado en cuatro pasos: cargás el complejo, compartís tu link, recibís reservas con seña y abrís la caja.","step":[{"@type":"HowToStep","position":1,"name":"Cargá tu complejo","text":"Canchas, horarios y precios. Conectás MercadoPago y las señas online quedan andando."},{"@type":"HowToStep","position":2,"name":"Compartí tu link","text":"Tu complejo tiene su propia página. Pegala en Instagram, en Google Maps y en el WhatsApp."},{"@type":"HowToStep","position":3,"name":"Dejá que reserven solos","text":"Entran reservas con seña, a cualquier hora. Las ves en la grilla, con lo que falta cobrar."},{"@type":"HowToStep","position":4,"name":"Abrí la caja y vendé","text":"Cobrás lo que falta, vendés lo del bar y al cerrar el turno la cuenta ya está hecha."}]}`,
/* El FAQPage se arma desde la misma fuente que renderiza la FAQ visible. Antes
   estaban las nueve respuestas escritas de nuevo acá, y Google descuenta el
   structured data que no coincide con lo que dice la pagina. */
JSON.stringify({
  '@context': 'https://schema.org',
  '@type': 'FAQPage',
  mainEntity: faq.map(item => ({
    '@type': 'Question',
    name: item.question,
    acceptedAnswer: { '@type': 'Answer', text: item.answer },
  })),
}, null, 2),
`   {"@context":"https://schema.org","@type":"Organization","name":"Vibe","url":"https://vibe.com.ar","logo":"https://vibe.com.ar/logo.png","description":"Sistema de gestión gratis para complejos deportivos en Argentina: reservas, cobros, caja, ventas y reportes.","areaServed":{"@type":"Country","name":"Argentina"},"foundingDate":"2026","email":"hola@vibe.com.ar","telephone":"+5491124638281","contactPoint":{"@type":"ContactPoint","contactType":"customer support","email":"hola@vibe.com.ar","telephone":"+5491124638281","areaServed":"AR","availableLanguage":["Spanish"]},"sameAs":["https://www.instagram.com/vibe.com.ar/","https://www.linkedin.com/company/vibe-reservas/","https://www.youtube.com/@Vibe-reservas","https://www.facebook.com/profile.php?id=61593795719275"]}`,
`   {"@context":"https://schema.org","@type":"WebSite","name":"Vibe","url":"https://vibe.com.ar","description":"Sistema de gestión para complejos deportivos, 100% gratis para el complejo: reservas, cobros, caja y ventas.","inLanguage":"es-AR","dateModified":"${LAST_MODIFIED}"}`,
];

export const privacidadJsonLd: string[] = [
`   {"@context":"https://schema.org","@type":"WebPage","name":"Política de Privacidad","description":"Cómo Vibe recopila, usa y protege los datos de los dueños de complejos y de quienes reservan, y cómo pedir que los corrijamos o borremos.","url":"https://vibe.com.ar/privacidad","inLanguage":"es-AR","isPartOf":{"@type":"WebSite","name":"Vibe","url":"https://vibe.com.ar"},"dateModified":"${LAST_MODIFIED}","publisher":{"@type":"Organization","name":"Vibe","url":"https://vibe.com.ar","logo":"https://vibe.com.ar/logo.png"}}`,
`   {"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[{"@type":"ListItem","position":1,"name":"Inicio","item":"https://vibe.com.ar"},{"@type":"ListItem","position":2,"name":"Política de Privacidad","item":"https://vibe.com.ar/privacidad"}]}`,
];

export const terminosJsonLd: string[] = [
`   {"@context":"https://schema.org","@type":"WebPage","name":"Términos y Condiciones","description":"Los términos de uso de Vibe, el sistema de gestión para complejos deportivos: reservas, cobros, caja, ventas y reportes, y qué hace cada parte.","url":"https://vibe.com.ar/terminos","inLanguage":"es-AR","isPartOf":{"@type":"WebSite","name":"Vibe","url":"https://vibe.com.ar"},"dateModified":"${LAST_MODIFIED}","publisher":{"@type":"Organization","name":"Vibe","url":"https://vibe.com.ar","logo":"https://vibe.com.ar/logo.png"}}`,
`   {"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[{"@type":"ListItem","position":1,"name":"Inicio","item":"https://vibe.com.ar"},{"@type":"ListItem","position":2,"name":"Términos y Condiciones","item":"https://vibe.com.ar/terminos"}]}`,
];

/* ---------------------------------------------------------------- guías --- */

/**
 * Una guía declara tres cosas: que es un artículo con autor y fecha, que las
 * preguntas del final son preguntas, y dónde vive dentro del sitio.
 *
 * `citation` apunta a la fuente primaria de los números que la guía afirma. Es
 * de lo poco con efecto medido sobre las citas en motores generativos, y acá no
 * cuesta nada porque la fuente ya está en el dato.
 */
export function guiaJsonLd(guia: Guia): string[] {
  const url = `https://vibe.com.ar/guias/${guia.slug}`;
  const editor = {
    '@type': 'Organization',
    name: 'Vibe',
    url: 'https://vibe.com.ar',
    logo: 'https://vibe.com.ar/logo.png',
  };

  const articulo: Record<string, unknown> = {
    '@context': 'https://schema.org',
    '@type': 'Article',
    headline: guia.title,
    description: guia.metaDescription,
    /* El primer párrafo responde la pregunta entera: es lo que conviene que se
       cite si se cita una sola cosa de la página. */
    abstract: guia.respuesta,
    url,
    mainEntityOfPage: { '@type': 'WebPage', '@id': url },
    inLanguage: 'es-AR',
    datePublished: guia.datePublished,
    dateModified: guia.dateModified,
    author: editor,
    publisher: editor,
    isPartOf: { '@type': 'WebSite', name: 'Vibe', url: 'https://vibe.com.ar' },
  };
  if (guia.fuente) {
    articulo.citation = {
      '@type': 'CreativeWork',
      name: guia.fuente.nombre,
      url: guia.fuente.url,
    };
  }

  const faq = {
    '@context': 'https://schema.org',
    '@type': 'FAQPage',
    mainEntity: guia.faq.map(f => ({
      '@type': 'Question',
      name: f.question,
      acceptedAnswer: { '@type': 'Answer', text: f.answer },
    })),
  };

  const breadcrumb = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: [
      { '@type': 'ListItem', position: 1, name: 'Inicio', item: 'https://vibe.com.ar' },
      { '@type': 'ListItem', position: 2, name: 'Guías', item: 'https://vibe.com.ar/guias' },
      { '@type': 'ListItem', position: 3, name: guia.title, item: url },
    ],
  };

  return [articulo, faq, breadcrumb].map(o => JSON.stringify(o, null, 2));
}

/** El índice: una lista de las guías, en orden. */
export function guiasIndexJsonLd(lista: Guia[]): string[] {
  return [
    JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'CollectionPage',
      name: 'Guías de Vibe',
      description: 'Respuestas directas, con números y fuente, a lo que te preguntás antes de ordenar la gestión de tu complejo deportivo.',
      url: 'https://vibe.com.ar/guias',
      inLanguage: 'es-AR',
      dateModified: LAST_MODIFIED,
      mainEntity: {
        '@type': 'ItemList',
        itemListElement: lista.map((g, i) => ({
          '@type': 'ListItem',
          position: i + 1,
          name: g.title,
          url: `https://vibe.com.ar/guias/${g.slug}`,
        })),
      },
    }, null, 2),
    JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'BreadcrumbList',
      itemListElement: [
        { '@type': 'ListItem', position: 1, name: 'Inicio', item: 'https://vibe.com.ar' },
        { '@type': 'ListItem', position: 2, name: 'Guías', item: 'https://vibe.com.ar/guias' },
      ],
    }, null, 2),
  ];
}

/* -------------------------------------------------------------- tutoriales */

/**
 * "30" -> "PT30S". Los tutoriales de hoy duran todos menos de un minuto, pero
 * la conversión es general para que un video más largo no rompa el schema el
 * día que aparezca.
 */
function duracionIso(segundos: number): string {
  const minutos = Math.floor(segundos / 60);
  const resto = segundos % 60;
  return `PT${minutos ? `${minutos}M` : ''}${resto || !minutos ? `${resto}S` : ''}`;
}

/**
 * Un tutorial declara que es un video (con su propio archivo, no el de
 * YouTube, que va en `sameAs`), que las preguntas del final son preguntas, y
 * dónde vive dentro del sitio. No lleva `citation`: a diferencia de una guía,
 * no afirma un número de un tercero, así que no hay fuente que citar.
 */
export function tutorialJsonLd(t: Tutorial): string[] {
  const url = `https://vibe.com.ar/tutoriales/${t.slug}`;
  const editor = {
    '@type': 'Organization',
    name: 'Vibe',
    url: 'https://vibe.com.ar',
    logo: 'https://vibe.com.ar/logo.png',
  };

  const video: Record<string, unknown> = {
    '@context': 'https://schema.org',
    '@type': 'VideoObject',
    /* El título SEO trae el sufijo " | Vibe" para el <title>; el schema
       describe el video en sí, así que se saca. */
    name: t.seoTitle.replace(/ \| Vibe$/, ''),
    description: t.metaDescription,
    thumbnailUrl: `${CDN}/${t.slug}.webp`,
    uploadDate: t.datePublished,
    duration: duracionIso(t.duracion),
    contentUrl: `${CDN}/${t.slug}.mp4`,
    inLanguage: 'es-AR',
    publisher: editor,
    isPartOf: { '@type': 'WebSite', name: 'Vibe', url: 'https://vibe.com.ar' },
  };
  /* `sameAs` solo declara la copia en YouTube cuando existe de verdad: un
     tutorial recién grabado que todavía no se subió ahí no lleva un enlace
     a un video que no está. */
  if (t.youtube) video.sameAs = `https://www.youtube.com/watch?v=${t.youtube}`;

  const faqSchema = {
    '@context': 'https://schema.org',
    '@type': 'FAQPage',
    mainEntity: t.faq.map(f => ({
      '@type': 'Question',
      name: f.question,
      acceptedAnswer: { '@type': 'Answer', text: f.answer },
    })),
  };

  const breadcrumb = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: [
      { '@type': 'ListItem', position: 1, name: 'Inicio', item: 'https://vibe.com.ar' },
      { '@type': 'ListItem', position: 2, name: 'Tutoriales', item: 'https://vibe.com.ar/tutoriales' },
      { '@type': 'ListItem', position: 3, name: t.title, item: url },
    ],
  };

  return [video, faqSchema, breadcrumb].map(o => JSON.stringify(o, null, 2));
}

/** El índice: una lista de los tutoriales, en orden. */
export function tutorialesIndexJsonLd(lista: Tutorial[]): string[] {
  return [
    JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'CollectionPage',
      name: 'Tutoriales de Vibe',
      description: 'Videos cortos del panel de Vibe, una pantalla por video: la grilla, los precios, los clientes, el panel, los reportes y la configuración.',
      url: 'https://vibe.com.ar/tutoriales',
      inLanguage: 'es-AR',
      dateModified: TUTORIALES_ULTIMA_ACTUALIZACION,
      mainEntity: {
        '@type': 'ItemList',
        itemListElement: lista.map((t, i) => ({
          '@type': 'ListItem',
          position: i + 1,
          name: t.title,
          url: `https://vibe.com.ar/tutoriales/${t.slug}`,
        })),
      },
    }, null, 2),
    JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'BreadcrumbList',
      itemListElement: [
        { '@type': 'ListItem', position: 1, name: 'Inicio', item: 'https://vibe.com.ar' },
        { '@type': 'ListItem', position: 2, name: 'Tutoriales', item: 'https://vibe.com.ar/tutoriales' },
      ],
    }, null, 2),
  ];
}
