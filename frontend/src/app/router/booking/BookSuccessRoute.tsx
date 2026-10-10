import { BookSuccessEntry } from '@vibe/booking/book-success';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getPublicBookingConfig } from '../publicBookingRuntime';
import { useRouteSlug } from './useRouteSlug';

const t = ES_AR;

export function BookSuccessRoute() {
  usePageTitle(t.publicBooking.bookingSuccess);
  const slug = useRouteSlug();
  return <BookSuccessEntry slug={slug} config={getPublicBookingConfig()} toaster={false} />;
}
