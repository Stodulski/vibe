import { useEffect } from 'react';

const APP_NAME = 'Vibe';

export function usePageTitle(title?: string) {
  useEffect(() => {
    document.title = title ? `${title} - ${APP_NAME}` : APP_NAME;
    return () => {
      document.title = APP_NAME;
    };
  }, [title]);
}

function setMetaProperty(property: string, content: string) {
  let el = document.querySelector(`meta[property="${property}"]`);
  if (!el) {
    el = document.createElement('meta');
    el.setAttribute('property', property);
    document.head.appendChild(el);
  }
  el.setAttribute('content', content);
}

function setMetaName(name: string, content: string) {
  let el = document.querySelector(`meta[name="${name}"]`);
  if (!el) {
    el = document.createElement('meta');
    el.setAttribute('name', name);
    document.head.appendChild(el);
  }
  el.setAttribute('content', content);
}

function resetMetaProperty(property: string, fallback: string) {
  const el = document.querySelector(`meta[property="${property}"]`);
  if (el) el.setAttribute('content', fallback);
}

function resetMetaName(name: string, fallback: string) {
  const el = document.querySelector(`meta[name="${name}"]`);
  if (el) el.setAttribute('content', fallback);
}

const OG_DEFAULTS = {
  title: 'Vibe - Reserva tu cancha',
  description: 'Reserva canchas de padel, tenis y futbol de forma rapida y segura.',
  image: '/logo.png',
};

export function useOGTags(og: {
  title: string;
  description: string;
  image?: string | undefined;
  url?: string | undefined;
}) {
  useEffect(() => {
    // Open Graph
    setMetaProperty('og:title', og.title);
    setMetaProperty('og:description', og.description);
    if (og.image) setMetaProperty('og:image', og.image);
    if (og.url) setMetaProperty('og:url', og.url);

    // Standard meta description
    setMetaName('description', og.description);

    // Twitter cards
    setMetaName('twitter:title', og.title);
    setMetaName('twitter:description', og.description);
    if (og.image) setMetaName('twitter:image', og.image);

    return () => {
      resetMetaProperty('og:title', OG_DEFAULTS.title);
      resetMetaProperty('og:description', OG_DEFAULTS.description);
      resetMetaProperty('og:image', OG_DEFAULTS.image);
      resetMetaName('description', OG_DEFAULTS.description);
      resetMetaName('twitter:title', OG_DEFAULTS.title);
      resetMetaName('twitter:description', OG_DEFAULTS.description);
      resetMetaName('twitter:image', OG_DEFAULTS.image);
    };
  }, [og.title, og.description, og.image, og.url]);
}
