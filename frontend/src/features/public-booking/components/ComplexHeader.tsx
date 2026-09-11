import { lazy, Suspense } from 'react';
import type { PublicComplex, Schedule } from '@/shared/types/api.types';
import { Skeleton } from '@/shared/components/ui/skeleton';
import { CoverBanner } from './complex-header/CoverBanner';
import { ComplexLogo } from './complex-header/ComplexLogo';
import { ComplexInfoRow } from './complex-header/ComplexInfoRow';
import { ComplexDetails } from './complex-header/ComplexDetails';

const ComplexMap = lazy(() => import('./ComplexMap').then((m) => ({ default: m.ComplexMap })));

interface ComplexHeaderProps {
  complex: PublicComplex;
  schedules: Schedule[];
  /** Forwarded to `ComplexDetails` -> `WeekScheduleList`, so the schedule bolds the day the slot picker is showing, not always today. */
  selectedDate?: Date;
}

/**
 * The club's own page: who they are, when they are open, what they have.
 *
 * This was a social profile — a cover band, a logo overlapping it, a name,
 * and one cramped line of address, phone and today's hours. It looked like
 * somewhere to follow rather than somewhere to book, and it withheld the two
 * things a player actually checks before choosing a club: whether it is open
 * on the day they want, and whether it has what they need.
 *
 * One column at every width. WHO the club is — cover, logo over its corner,
 * name, contact — comes first and keeps the same internal arrangement at
 * every width. WHAT the club offers — hours and services — follows underneath,
 * spanning the full width of the card, so the week's hours fit on one line
 * and the services wrap into a few short rows.
 *
 * This used to split into two columns from `lg`, identity left and details
 * right. The split was dropped so the details get the whole width instead of
 * a 56% column.
 */
export function ComplexHeader({ complex, schedules, selectedDate }: ComplexHeaderProps) {
  const coverUrl = complex.cover_url ?? null;

  return (
    // The bordered card arrives at `lg`. Below that a panel spanning the whole
    // screen frames the page and then indents its contents inside the page's
    // own gutter — two nested paddings for one column of content.
    <div className="relative lg:rounded-2xl lg:border lg:border-border-subtle lg:bg-bg-subtle lg:p-7">
      {/* Identity: cover, logo, name, contact — one block, one order. */}
      <div>
        <CoverBanner coverUrl={coverUrl} />

        <div className="relative pb-5 sm:pb-7">
          <ComplexLogo logoUrl={complex.logo_url ?? null} name={complex.name} />

          <h1 className="font-display text-xl font-bold tracking-tight text-text-primary sm:text-2xl">
            {complex.name}
          </h1>

          <ComplexInfoRow complex={complex} />
        </div>
      </div>

      {/* What it offers. */}
      <div>
        <ComplexDetails schedules={schedules} amenities={complex.amenities} selectedDate={selectedDate} />

        {complex.latitude != null && complex.longitude != null && (
          <div className="mt-6">
            <Suspense fallback={<Skeleton className="h-[200px] w-full rounded-xl" />}>
              <ComplexMap
                latitude={complex.latitude}
                longitude={complex.longitude}
                name={complex.name}
                address={`${complex.address}, ${complex.city}`}
              />
            </Suspense>
          </div>
        )}
      </div>
    </div>
  );
}
