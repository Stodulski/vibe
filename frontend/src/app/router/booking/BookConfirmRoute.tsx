import { BookConfirmEntry } from '@vibe/booking/book-confirm';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getPublicBookingConfig } from '../publicBookingRuntime';
import { useRouteSlug } from './useRouteSlug';

const t = ES_AR;

export function BookConfirmRoute() {
  usePageTitle(t.publicBooking.confirmBooking);
  const slug = useRouteSlug();
  return <BookConfirmEntry slug={slug} config={getPublicBookingConfig()} toaster={false} />;
}
