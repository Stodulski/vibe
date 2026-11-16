import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { toDisplayDate } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { categoryTitle, tournamentLabel } from '../../lib/traitLabels';
import type { PublicTournament, PublicTournamentCategory } from '../../api/types';
import { SignUpSummaryCard } from '../signup/SignUpSummaryCard';
import { formatLabel, matchFormatLabel, sportLabel, tournamentDates } from './tournamentModel';

const t = BOOKING_MESSAGES;

interface PairSummaryProps {
  tournament: PublicTournament;
  category: PublicTournamentCategory;
  /** The complex, on the ticket's first line. */
  venue?: string | undefined;
  /** The window the pair is told before paying, when the club sets one. */
  cancellationHours?: number | undefined;
}

/**
 * The ticket of what the pair pays for, in the same shape as a booking's: the
 * complex and the category's day, the tournament's label above the category's
 * title, the format and the sport as chips, then the price rows and the
 * cancellation window. The checkout page sets it in its green band, above the form.
 */
export function PairSummary({ tournament, category, venue, cancellationHours }: PairSummaryProps) {
  // Like a booking's ticket: the day as a line, the start time large.
  const dateLabel = category.starts_on
    ? format(toDisplayDate(category.starts_on), "EEEE d 'de' MMMM", { locale: es })
    : tournamentDates(tournament);
  const timeLabel = category.starts_on ? (category.starts_at ?? undefined) : undefined;
  return (
    <SignUpSummaryCard
      bare
      ariaLabel={t.tournaments.summaryLabel}
      venue={venue}
      dateLabel={dateLabel}
      timeLabel={timeLabel}
      kicker={tournamentLabel(tournament.sport)}
      title={categoryTitle(category, tournament.sport)}
      tags={[formatLabel(category.format), sportLabel(tournament.sport)]}
      details={[matchFormatLabel(category)]}
      priceLabel={t.tournaments.priceLabel}
      price={category.price}
      serviceFee={category.service_fee ?? 0}
      total={category.total_price ?? category.price + (category.service_fee ?? 0)}
      feeNote={t.tournaments.feeNote}
      cancellationHours={cancellationHours}
    />
  );
}
