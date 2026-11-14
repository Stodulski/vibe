import { BookingRoot } from '../BookingRoot';
import TournamentPage from '../pages/TournamentPage';
import type { BookingEntryProps } from './types';

/** The public page of one tournament, and where the checkout returns to. */
export function TournamentPageEntry({
  slug,
  tournamentId,
  config,
  initialSearch,
  toaster,
  children,
}: BookingEntryProps & { tournamentId: string }) {
  return (
    <BookingRoot config={config} initialSearch={initialSearch} toaster={toaster}>
      {children}
      <TournamentPage slug={slug} tournamentId={tournamentId} />
    </BookingRoot>
  );
}
