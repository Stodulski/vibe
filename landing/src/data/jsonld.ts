/* JSON-LD payloads copied verbatim from the pre-Astro static HTML. Each page's
   modification date is its own (src/data/page-dates.json), so it moves when that page
   changes and not on every commit. The organization, the website and the software are
   defined once, with an @id, and every other node points at them by that @id. */
import { PUBLISHED, SITE_MODIFIED, pageDateModified } from './site-dates.ts';
import type { Guia } from './guias.ts';
import type { Tutorial } from './tutoriales.ts';
import { ULTIMA_ACTUALIZACION as TUTORIALES_ULTIMA_ACTUALIZACION } from './tutoriales.ts';
import { faq } from './faq.ts';

/* Un Article pide `image` absoluta. Las guías y los tutoriales no tienen imagen
   propia, así que van con la del sitio, que ya mide 1200x630. */
const IMAGEN_ARTICULO = 'https://vibe.com.ar/og-image.png';

/* The entities of the site graph. Defined once on the home page; every other node
   names them by @id. */
const ORGANIZATION_ID = 'https://vibe.com.ar/#organization';
const WEBSITE_ID = 'https://vibe.com.ar/#website';
const SOFTWARE_ID = 'https://vibe.com.ar/#software';
const ORGANIZATION_REF = { '@id': ORGANIZATION_ID };
const WEBSITE_REF = { '@id': WEBSITE_ID };

export const homeJsonLd: string[] = [
`   {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "@id": "${SOFTWARE_ID}",
    "name": "Vibe",
    "description": "Sistema de gestión gratis para complejos deportivos: cobros en el mostrador, caja por turno con arqueo, venta de productos con stock, clientes, reportes y reservas online con seña.",
    "url": "https://vibe.com.ar/",
    "applicationCategory": "BusinessApplication",
    "operatingSystem": "Web",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "ARS",
      "availability": "https://schema.org/InStock",
      "description": "Gratis para el complejo deportivo: sin costo fijo, sin suscripción y sin cargo sobre los cobros en el mostrador ni las ventas."
    },
    "featureList": [
      "Cobros en el mostrador: efectivo, transferencia, débito, crédito y QR",
      "Caja por turno con arqueo de efectivo",
      "Ingresos y egresos por categoría",
      "Venta de productos con control de stock",
      "Aviso de stock bajo",
      "Recordatorio automático 2 horas antes del turno",
      "Base de clientes con asistencia y bloqueo",
      "Dashboard con ocupación por horario",
      "Reporte mensual con exportación a Excel",
      "Reservas online 24/7 con página pública del complejo",
      "Cobro de señas con MercadoPago y reembolso automático",
      "Precios por cancha, día y franja horaria"
    ],
    "author": ${JSON.stringify(ORGANIZATION_REF)},
    "publisher": ${JSON.stringify(ORGANIZATION_REF)},
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
    "dateModified": "${pageDateModified('/')}"
  }`,
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
`   {"@context":"https://schema.org","@type":"Organization","@id":"${ORGANIZATION_ID}","name":"Vibe","url":"https://vibe.com.ar/","logo":"https://vibe.com.ar/logo.png","description":"Sistema de gestión gratis para complejos deportivos en Argentina: reservas, cobros, caja, ventas y reportes.","areaServed":{"@type":"Country","name":"Argentina"},"foundingDate":"2026","email":"hola@vibe.com.ar","telephone":"+5491124638281","contactPoint":{"@type":"ContactPoint","contactType":"customer support","email":"hola@vibe.com.ar","telephone":"+5491124638281","areaServed":"AR","availableLanguage":["Spanish"]},"sameAs":["https://www.instagram.com/vibe.com.ar/","https://www.linkedin.com/company/vibe-reservas/","https://www.youtube.com/@Vibe-reservas","https://www.facebook.com/profile.php?id=61593795719275"]}`,
`   {"@context":"https://schema.org","@type":"WebSite","@id":"${WEBSITE_ID}","name":"Vibe","url":"https://vibe.com.ar/","description":"Sistema de gestión para complejos deportivos, gratis para el complejo: reservas, cobros, caja y ventas.","inLanguage":"es-AR","publisher":{"@id":"${ORGANIZATION_ID}"},"dateModified":"${SITE_MODIFIED}"}`,
];

export const privacidadJsonLd: string[] = [
`   {"@context":"https://schema.org","@type":"WebPage","name":"Política de Privacidad","description":"Cómo Vibe recopila, usa y protege los datos de los dueños de complejos y de quienes reservan, y cómo pedir que los corrijamos o borremos.","url":"https://vibe.com.ar/privacidad","inLanguage":"es-AR","isPartOf":${JSON.stringify(WEBSITE_REF)},"dateModified":"${pageDateModified('/privacidad')}","publisher":${JSON.stringify(ORGANIZATION_REF)}}`,
`   {"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[{"@type":"ListItem","position":1,"name":"Inicio","item":"https://vibe.com.ar/"},{"@type":"ListItem","position":2,"name":"Política de Privacidad","item":"https://vibe.com.ar/privacidad"}]}`,
];

export const terminosJsonLd: string[] = [
`   {"@context":"https://schema.org","@type":"WebPage","name":"Términos y Condiciones","description":"Los términos de uso de Vibe, el sistema de gestión para complejos deportivos: reservas, cobros, caja, ventas y reportes, y qué hace cada parte.","url":"https://vibe.com.ar/terminos","inLanguage":"es-AR","isPartOf":${JSON.stringify(WEBSITE_REF)},"dateModified":"${pageDateModified('/terminos')}","publisher":${JSON.stringify(ORGANIZATION_REF)}}`,
`   {"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[{"@type":"ListItem","position":1,"name":"Inicio","item":"https://vibe.com.ar/"},{"@type":"ListItem","position":2,"name":"Términos y Condiciones","item":"https://vibe.com.ar/terminos"}]}`,
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

  const articulo: Record<string, unknown> = {
    '@context': 'https://schema.org',
    '@type': 'Article',
    headline: guia.title,
    description: guia.metaDescription,
    image: IMAGEN_ARTICULO,
    /* El primer párrafo responde la pregunta entera: es lo que conviene que se
       cite si se cita una sola cosa de la página. */
    abstract: guia.respuesta,
    url,
    mainEntityOfPage: { '@type': 'WebPage', '@id': url },
    inLanguage: 'es-AR',
    datePublished: guia.datePublished,
    dateModified: guia.dateModified,
    author: ORGANIZATION_REF,
    publisher: ORGANIZATION_REF,
    isPartOf: WEBSITE_REF,
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
      { '@type': 'ListItem', position: 1, name: 'Inicio', item: 'https://vibe.com.ar/' },
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
      description: 'Respuestas directas, con números y fuente, a lo que te preguntás para hacer crecer tu complejo deportivo.',
      url: 'https://vibe.com.ar/guias',
      inLanguage: 'es-AR',
      dateModified: pageDateModified('/guias'),
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
        { '@type': 'ListItem', position: 1, name: 'Inicio', item: 'https://vibe.com.ar/' },
        { '@type': 'ListItem', position: 2, name: 'Guías', item: 'https://vibe.com.ar/guias' },
      ],
    }, null, 2),
  ];
}

/* -------------------------------------------------------------- tutoriales */

/**
 * Un tutorial declara que es un artículo (misma forma que una guía), que las
 * preguntas del final son preguntas, y dónde vive dentro del sitio. No lleva
 * `citation`: a diferencia de una guía, no afirma un número de un tercero, así
 * que no hay fuente que citar.
 */
export function tutorialJsonLd(t: Tutorial): string[] {
  const url = `https://vibe.com.ar/tutoriales/${t.slug}`;

  const articulo: Record<string, unknown> = {
    '@context': 'https://schema.org',
    '@type': 'Article',
    headline: t.title,
    description: t.metaDescription,
    image: IMAGEN_ARTICULO,
    /* La respuesta contesta la pregunta entera: es lo que conviene que se cite
       si se cita una sola cosa de la página. */
    abstract: t.respuesta,
    url,
    mainEntityOfPage: { '@type': 'WebPage', '@id': url },
    inLanguage: 'es-AR',
    datePublished: t.datePublished,
    dateModified: t.dateModified,
    author: ORGANIZATION_REF,
    publisher: ORGANIZATION_REF,
    isPartOf: WEBSITE_REF,
  };

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
      { '@type': 'ListItem', position: 1, name: 'Inicio', item: 'https://vibe.com.ar/' },
      { '@type': 'ListItem', position: 2, name: 'Tutoriales', item: 'https://vibe.com.ar/tutoriales' },
      { '@type': 'ListItem', position: 3, name: t.title, item: url },
    ],
  };

  return [articulo, faqSchema, breadcrumb].map(o => JSON.stringify(o, null, 2));
}

/** El índice: una lista de los tutoriales, en orden. */
export function tutorialesIndexJsonLd(lista: Tutorial[]): string[] {
  return [
    JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'CollectionPage',
      name: 'Tutoriales de Vibe',
      description: 'Tutoriales del panel de Vibe, pantalla por pantalla: grilla, precios, clientes, caja, ventas, stock, reportes y configuración.',
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
        { '@type': 'ListItem', position: 1, name: 'Inicio', item: 'https://vibe.com.ar/' },
        { '@type': 'ListItem', position: 2, name: 'Tutoriales', item: 'https://vibe.com.ar/tutoriales' },
      ],
    }, null, 2),
  ];
}
