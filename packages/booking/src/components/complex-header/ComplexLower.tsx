import { lazy, Suspense, useContext } from 'react';
import { ErrorBoundary, Skeleton } from '@vibe/ui';
import type { PublicComplex } from '../../api/types';
import { BookingConfigContext } from '../../config';
import { useHasMounted } from '../../hooks/useHasMounted';
import { orderedAmenities } from '../../lib/amenities';
import { BOOKING_MESSAGES } from '../../messages';
import { AmenityList } from './AmenityList';
import { Section } from './ComplexDetails';

const t = BOOKING_MESSAGES;

const ComplexMap = lazy(() => import('../ComplexMap').then((m) => ({ default: m.ComplexMap })));

/**
 * The services and the location of the club: what a visitor checks once the
 * hours are known.
 *
 * It has one place on the page at a time. In the open card at `lg` it sits
 * after the hours, inside the header. Below `lg` the page puts it after the
 * booking, so the slots are the first thing under the club card. Either way the
 * DOM order is the visual order, and the page never renders it twice.
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
    <div className="space-y-6">
      {hasAmenities && (
        <Section title={t.complex.amenitiesSection}>
          <AmenityList amenities={complex.amenities} />
        </Section>
      )}

      {complex.latitude != null && complex.longitude != null && (
        <div>
          {/*
            Leaflet is a third-party map widget loaded lazily; a failure in it (a
            bad tile server, a bundling issue) must not take down the rest of the
            club's page with it — see routeHelpers.tsx's own per-page ErrorBoundary
            for the equivalent guard one level up.
          */}
          <ErrorBoundary
            onError={(error, info) => reportError?.(error, { componentStack: info.componentStack })}
            fallback={
              <div className="border-border-subtle bg-bg-subtle text-text-secondary flex h-[200px] w-full items-center justify-center rounded-xl border text-center text-sm">
                {t.publicBooking.mapUnavailable}
              </div>
            }
          >
            {mapMounted ? (
              <Suspense fallback={<Skeleton className="h-[200px] w-full rounded-xl" />}>
                <ComplexMap
                  latitude={complex.latitude}
                  longitude={complex.longitude}
                  name={complex.name}
                  address={`${complex.address}, ${complex.city}`}
                />
              </Suspense>
            ) : (
              <Skeleton className="h-[200px] w-full rounded-xl" />
            )}
          </ErrorBoundary>
        </div>
      )}
    </div>
  );
}
