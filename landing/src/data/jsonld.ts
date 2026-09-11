/* JSON-LD payloads copied verbatim from the pre-Astro static HTML, with the
   modification dates wired to the last commit so they cannot go stale again. */
import { LAST_MODIFIED, PUBLISHED } from './site-dates.ts';
import type { Guia } from './guias.ts';
import { faq } from './faq.ts';
import { RANGO_NACIONAL } from './mercadopago-costos.ts';
import { CARGO_SERVICIO_TEXTO, CARGO_MINIMO, pesos } from './precio.ts';

export const homeJsonLd: string[] = [
`   {
    "@context": "https://schema.org",
    "@type": "SoftwareApplication",
    "name": "Vibe",
    "description": "Plataforma de gestión de complejos deportivos y reservas online. Administrá tus canchas, recibí reservas y cobrá señas de forma automática.",
    "url": "https://vibe.com.ar",
    "applicationCategory": "BusinessApplication",
    "operatingSystem": "Web",
    "offers": {
      "@type": "Offer",
      "price": "0",
      "priceCurrency": "ARS",
      "availability": "https://schema.org/InStock",
      "description": "Gratis para el complejo deportivo: $0 de costo fijo, sin suscripción. Al cliente que reserva se le suma un cargo de servicio del ${CARGO_SERVICIO_TEXTO} sobre la seña, con un mínimo de ${pesos(CARGO_MINIMO)} ARS. La comisión de MercadoPago sobre la seña la fija MercadoPago y va ${RANGO_NACIONAL} más IVA, según la provincia del complejo y cuándo se acredite el dinero."
    },
    "featureList": [
      "Reservas online 24/7",
      "Panel de administración",
      "Cobro automático con MercadoPago",
      "Precios flexibles",
      "Recordatorios automáticos",
      "Gestión de clientes",
      "Estadísticas y métricas",
      "Múltiples complejos"
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
      "name": "Complejos deportivos de pádel, tenis, fútbol y básquet"
    },
    "datePublished": "${PUBLISHED}",
    "dateModified": "${LAST_MODIFIED}"
  }`,
`   {"@context":"https://schema.org","@type":"HowTo","name":"Cómo funciona Vibe en tu complejo deportivo","description":"Así se configura, en menos de 5 minutos, para recibir reservas y cobros automáticos.","totalTime":"PT5M","step":[{"@type":"HowToStep","position":1,"name":"Configurá tu complejo","text":"Cargá tus canchas, horarios y precios paso a paso, y conectá MercadoPago para cobrar señas online."},{"@type":"HowToStep","position":2,"name":"Compartí tu link","text":"Tu complejo tiene su página pública. Compartila en Instagram, Google Maps o donde quieras."},{"@type":"HowToStep","position":3,"name":"Recibí reservas","text":"Tus clientes reservan y pagan solos. Vos gestionás todo desde el panel sin levantar el teléfono."}]}`,
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
`   {
    "@context": "https://schema.org",
    "@type": "VideoObject",
    "name": "Cómo un cliente reserva y paga una cancha en Vibe",
    "description": "Un cliente abre la página pública del complejo desde el celular, elige cancha y horario, deja sus datos y paga la seña con MercadoPago. La reserva queda confirmada sin que el complejo tenga que intervenir.",
    "thumbnailUrl": "https://vibe.com.ar/video/hero-reserva-poster.jpg",
    "contentUrl": "https://vibe.com.ar/video/hero-reserva.mp4",
    "uploadDate": "2026-08-21",
    "duration": "PT17S",
    "width": 430,
    "height": 986,
    "inLanguage": "es-AR",
    "isFamilyFriendly": true,
    "publisher": {
      "@type": "Organization",
      "name": "Vibe",
      "url": "https://vibe.com.ar"
    }
  }`,
`   {"@context":"https://schema.org","@type":"Organization","name":"Vibe","url":"https://vibe.com.ar","logo":"https://vibe.com.ar/logo.png","description":"Plataforma de gestión de complejos deportivos y reservas online en Argentina.","areaServed":{"@type":"Country","name":"Argentina"},"foundingDate":"2026","email":"hola@vibe.com.ar","telephone":"+5491124638281","contactPoint":{"@type":"ContactPoint","contactType":"customer support","email":"hola@vibe.com.ar","telephone":"+5491124638281","areaServed":"AR","availableLanguage":["Spanish"]},"sameAs":["https://www.instagram.com/vibe.com.ar/"]}`,
`   {"@context":"https://schema.org","@type":"WebSite","name":"Vibe","url":"https://vibe.com.ar","description":"Plataforma de gestión de complejos deportivos y reservas online.","inLanguage":"es-AR","dateModified":"${LAST_MODIFIED}"}`,
];

export const privacidadJsonLd: string[] = [
`   {"@context":"https://schema.org","@type":"WebPage","name":"Política de Privacidad","description":"Política de privacidad de Vibe. Cómo recopilamos, usamos y protegemos datos personales.","url":"https://vibe.com.ar/privacidad","inLanguage":"es-AR","isPartOf":{"@type":"WebSite","name":"Vibe","url":"https://vibe.com.ar"},"dateModified":"${LAST_MODIFIED}","publisher":{"@type":"Organization","name":"Vibe","url":"https://vibe.com.ar","logo":"https://vibe.com.ar/logo.png"}}`,
`   {"@context":"https://schema.org","@type":"BreadcrumbList","itemListElement":[{"@type":"ListItem","position":1,"name":"Inicio","item":"https://vibe.com.ar"},{"@type":"ListItem","position":2,"name":"Política de Privacidad","item":"https://vibe.com.ar/privacidad"}]}`,
];

export const terminosJsonLd: string[] = [
`   {"@context":"https://schema.org","@type":"WebPage","name":"Términos y Condiciones","description":"Términos y condiciones de uso de Vibe. Reglas que rigen el uso de la plataforma de gestión de reservas.","url":"https://vibe.com.ar/terminos","inLanguage":"es-AR","isPartOf":{"@type":"WebSite","name":"Vibe","url":"https://vibe.com.ar"},"dateModified":"${LAST_MODIFIED}","publisher":{"@type":"Organization","name":"Vibe","url":"https://vibe.com.ar","logo":"https://vibe.com.ar/logo.png"}}`,
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
      description: 'Respuestas concretas a lo que se pregunta antes de digitalizar un complejo deportivo.',
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
