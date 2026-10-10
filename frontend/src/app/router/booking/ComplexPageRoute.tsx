import { ComplexPageEntry } from '@vibe/booking/complex-page';
import { useComplexBySlug } from '@vibe/booking/complex-query';
import { getPublicBookingConfig } from '../publicBookingRuntime';
import { useComplexPageMeta } from '../useComplexPageMeta';
import { useRouteSlug } from './useRouteSlug';

/**
 * Head tags of the complex page. It renders inside the entry's BookingRoot, so it
 * reads the same cached complex the page reads.
 */
function ComplexPageHead({ slug }: { slug: string }) {
  const { data } = useComplexBySlug(slug);
  useComplexPageMeta(slug, data);
  return null;
}

// The entry mounts its own BookingRoot. The Toaster is off here because the
// app's Providers already mounts one.
export function ComplexPageRoute() {
  const slug = useRouteSlug();
  return (
    <ComplexPageEntry slug={slug} config={getPublicBookingConfig()} toaster={false}>
      <ComplexPageHead slug={slug} />
    </ComplexPageEntry>
  );
}
