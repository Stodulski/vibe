import type { CourtWithPrices, PublicComplex, Schedule } from '../api/types';
import { sportSummaries } from '../lib/courtPrices';
import { complexCoverAlt } from '../lib/complexImageAlt';
import { BOOKING_MESSAGES } from '../messages';
import { CoverBanner } from './complex-header/CoverBanner';
import { ComplexLogo } from './complex-header/ComplexLogo';
import { ComplexInfoRow } from './complex-header/ComplexInfoRow';
import { ShareComplexButton } from './complex-header/ShareComplexButton';
import { ComplexDetails } from './complex-header/ComplexDetails';
import { CourtSummary } from './complex-header/CourtSummary';

const t = BOOKING_MESSAGES;

interface ComplexHeaderProps {
  complex: PublicComplex;
  schedules: Schedule[];
  /** The active courts with their prices: the page states how many of each sport there are and from what hourly price. */
  courts?: readonly CourtWithPrices[];
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
 * every width. WHAT the club offers — the courts and their prices, then the
 * hours — follows underneath, spanning the full width of the card, so the
 * week's hours fit on one line.
 *
 * The services and the map are not in this card. The page places them after
 * the booking at every width (see `ComplexLower`), so the markup order is the
 * reading and focus order everywhere.
 *
 * This used to split into two columns from `lg`, identity left and details
 * right. The split was dropped so the details get the whole width instead of
 * a 56% column.
 */
export function ComplexHeader({ complex, schedules, courts = [], selectedDate }: ComplexHeaderProps) {
  const coverUrl = complex.cover_url ?? null;
  const coverAlt = complexCoverAlt({
    name: complex.name,
    city: complex.city,
    sports: sportSummaries(courts).map(({ sport }) => t.courts.sportTypes[sport]),
  });

  return (
    // The bordered card arrives at `lg`. Below that it is a plain block.
    <div className="lg:border-border-subtle lg:bg-bg-subtle relative lg:rounded-2xl lg:border lg:p-7">
      {/* Identity: cover, logo, name, contact — one block, one order. */}
      <div>
        <CoverBanner coverUrl={coverUrl} alt={coverAlt} />

        <div className="relative pb-5 sm:pb-7">
          <ComplexLogo logoUrl={complex.logo_url ?? null} name={complex.name} />

          <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">
            {complex.name}
          </h1>

          <ComplexInfoRow complex={complex} />

          <div className="mt-2">
            <ShareComplexButton name={complex.name} slug={complex.slug} />
          </div>
        </div>
      </div>

      {/* What it offers: the courts and their hourly prices, then the hours. */}
      <div>
        <CourtSummary courts={courts} />
        <ComplexDetails schedules={schedules} selectedDate={selectedDate} />
      </div>
    </div>
  );
}
