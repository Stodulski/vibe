import { usePageTitle, useOGTags, useCanonical, useStructuredData } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { env } from '@/shared/lib/env';
import { publicComplexPath } from '@/shared/lib/publicPaths';
import type { PublicComplex, Schedule } from '@/shared/types/api.types';
import { buildComplexSchema } from '@vibe/booking/complex-schema';

const t = ES_AR;

interface ComplexPageMetaData {
  complex: PublicComplex;
  schedules: Schedule[];
}

/**
 * Wires up document title, canonical URL, OG tags, and JSON-LD structured data
 * for a complex storefront. Lives in the app, not the booking package: head tags
 * are rendered by the server once the package is mounted as an island.
 */
export function useComplexPageMeta(slug: string | undefined, data: ComplexPageMetaData | undefined) {
  const appUrl = env.VITE_APP_URL;
  const canonicalUrl = `${appUrl}${publicComplexPath(String(slug))}`;
  const complexName = data?.complex.name;

  usePageTitle(complexName);
  useCanonical(canonicalUrl);
  useOGTags({
    title: complexName ? `${complexName} - ${t.publicBooking.bookYourCourt}` : 'Vibe',
    description: complexName
      ? `${t.publicBooking.bookCourtsAt} ${complexName}${t.publicBooking.bookCourtsAtSuffix}`
      : '',
    image: data?.complex.logo_url ?? undefined,
    url: canonicalUrl,
  });

  useStructuredData(data ? buildComplexSchema(data.complex, data.schedules, canonicalUrl) : null);

  return canonicalUrl;
}
