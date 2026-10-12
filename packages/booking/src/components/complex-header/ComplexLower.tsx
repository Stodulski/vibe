import { lazy, Suspense, useContext } from 'react';
import { ErrorBoundary, Skeleton } from '@vibe/ui';
import { ExternalLink } from 'lucide-react';
import type { PublicComplex } from '../../api/types';
import { BookingConfigContext } from '../../config';
import { useHasMounted } from '../../hooks/useHasMounted';
import { orderedAmenities } from '../../lib/amenities';
import { BOOKING_MESSAGES } from '../../messages';
import { AmenityList } from './AmenityList';
import { Section } from './ComplexDetails';

const t = BOOKING_MESSAGES;

const ComplexMap = lazy(() => import('../ComplexMap').then((m) => ({ default: m.ComplexMap })));

function googleMapsSearchUrl(latitude: number, longitude: number) {
  return `https://www.google.com/maps/search/?api=1&query=${String(latitude)},${String(longitude)}`;
}

/**
 * The services and the location of the club: what a visitor checks once the
 * hours are known.
 *
 * The page renders it once, after the booking, at every width. Its place in
 * the markup is its place on screen and in the focus order, so no CSS moves it.
 *
 * Renders nothing when the club has neither services nor coordinates.
 */
export function ComplexLower({ complex }: { complex: PublicComplex }) {
  const hasAmenities = orderedAmenities(complex.amenities).length > 0;
  const hasMap = complex.latitude != null && complex.longitude != null;
  // Optional: this also renders on its own (tests, previews), without a BookingRoot.
  const reportError = useContext(BookingConfigContext)?.reportError;
  // Leaflet touches `window` when it loads, so the map is requested only after
  // mount. The server markup and the first client render both show the skeleton.
  const mapMounted = useHasMounted();

  if (!hasAmenities && !hasMap) return null;

  return (
    <div className="mt-6 sm:mt-10">
      {hasAmenities && (
        <Section title={t.complex.amenitiesSection}>
          <AmenityList amenities={complex.amenities} />
        </Section>
      )}

      {complex.latitude != null && complex.longitude != null && (
        <div className="mt-6 first:mt-0">
          {/*
            Leaflet is a third-party map widget loaded lazily; a failure in it (a
            bad tile server, a bundling issue) must not take down the rest of the
            club's page with it — see routeHelpers.tsx's own per-page ErrorBoundary
            for the equivalent guard one level up.
          */}
          <div className="border-border-subtle overflow-hidden rounded-xl border">
            {mapMounted ? (
              <ErrorBoundary
                onError={(error, info) => reportError?.(error, { componentStack: info.componentStack })}
                fallback={
                  <div className="bg-bg-subtle text-text-secondary flex h-[200px] w-full items-center justify-center text-center text-sm">
                    {t.publicBooking.mapUnavailable}
                  </div>
                }
              >
                <Suspense fallback={<Skeleton className="h-[200px] w-full" />}>
                  <ComplexMap
                    latitude={complex.latitude}
                    longitude={complex.longitude}
                    name={complex.name}
                    address={`${complex.address}, ${complex.city}`}
                  />
                </Suspense>
              </ErrorBoundary>
            ) : (
              <Skeleton className="h-[200px] w-full" />
            )}
            <a
              href={googleMapsSearchUrl(complex.latitude, complex.longitude)}
              target="_blank"
              rel="noopener noreferrer"
              className="bg-bg-subtle text-primary-400 hover:bg-bg-base hover:text-primary-300 flex items-center justify-center gap-2 px-4 py-2.5 text-xs font-medium transition-colors"
            >
              <ExternalLink className="size-3.5" />
              {t.complex.viewOnGoogleMaps}
            </a>
          </div>
        </div>
      )}
    </div>
  );
}
