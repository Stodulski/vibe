import { useParams } from 'react-router-dom';
import { useComplexBySlug } from '@/features/public-booking';
import {
  BookCancelEntry,
  BookConfirmEntry,
  BookSuccessEntry,
  ComplexPageEntry,
  PaymentFailedEntry,
} from '@/features/public-booking/entries';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getPublicBookingConfig } from './publicBookingRuntime';
import { useComplexPageMeta } from './useComplexPageMeta';

const t = ES_AR;

/** The `:slug` of the route. The booking pages treat an empty slug as "no complex". */
function useRouteSlug(): string {
  const { slug } = useParams<{ slug: string }>();
  return slug ?? '';
}

/**
 * Head tags of the complex page. It renders inside the entry's BookingRoot, so it
 * reads the same cached complex the page reads.
 */
function ComplexPageHead({ slug }: { slug: string }) {
  const { data } = useComplexBySlug(slug);
  useComplexPageMeta(slug, data);
  return null;
}

// The entries mount their own BookingRoot. The Toaster is off here because the
// app's Providers already mounts one.

export function ComplexPageRoute() {
  const slug = useRouteSlug();
  return (
    <ComplexPageEntry slug={slug} config={getPublicBookingConfig()} toaster={false}>
      <ComplexPageHead slug={slug} />
    </ComplexPageEntry>
  );
}

export function BookPageRoute() {
  const slug = useRouteSlug();
  return <PaymentFailedEntry slug={slug} config={getPublicBookingConfig()} toaster={false} />;
}

export function BookConfirmRoute() {
  usePageTitle(t.publicBooking.confirmBooking);
  const slug = useRouteSlug();
  return <BookConfirmEntry slug={slug} config={getPublicBookingConfig()} toaster={false} />;
}

export function BookSuccessRoute() {
  usePageTitle(t.publicBooking.bookingSuccess);
  const slug = useRouteSlug();
  return <BookSuccessEntry slug={slug} config={getPublicBookingConfig()} toaster={false} />;
}

export function BookCancelRoute() {
  usePageTitle(t.publicBooking.cancelBookingQuestion);
  const slug = useRouteSlug();
  return <BookCancelEntry slug={slug} config={getPublicBookingConfig()} toaster={false} />;
}
