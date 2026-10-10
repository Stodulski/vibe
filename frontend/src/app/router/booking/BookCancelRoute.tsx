import { BookCancelEntry } from '@vibe/booking/book-cancel';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getPublicBookingConfig } from '../publicBookingRuntime';
import { useRouteSlug } from './useRouteSlug';

const t = ES_AR;

export function BookCancelRoute() {
  usePageTitle(t.publicBooking.cancelBookingQuestion);
  const slug = useRouteSlug();
  return <BookCancelEntry slug={slug} config={getPublicBookingConfig()} toaster={false} />;
}
