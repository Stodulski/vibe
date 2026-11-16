import { BookingRoot } from '../BookingRoot';
import TournamentSignUpPage from '../pages/TournamentSignUpPage';
import type { BookingEntryProps } from './types';

/** The pair sign-up of one category: `/c/<slug>/tournaments/<id>/inscripcion`, with the category in the query. */
export function TournamentSignUpEntry({
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
      <TournamentSignUpPage slug={slug} tournamentId={tournamentId} />
    </BookingRoot>
  );
}
