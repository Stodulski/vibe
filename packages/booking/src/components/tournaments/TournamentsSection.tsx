import { BOOKING_MESSAGES } from '../../messages';
import { TournamentAgenda } from './TournamentAgenda';
import type { AgendaRow } from './agendaModel';

const t = BOOKING_MESSAGES.tournaments;

/**
 * The complex's tournaments as an agenda, once "Torneos" is picked: one row per
 * category open for sign-up with a place left. The caller passes those rows: see
 * `agendaRows`.
 */
export function TournamentsSection({ rows, slug }: { rows: AgendaRow[]; slug: string }) {
  return (
    <section aria-labelledby="tournaments-title" className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h3 id="tournaments-title" className="text-text-primary text-xl font-bold">
          {t.sectionTitle}
        </h3>
        <p className="text-text-secondary text-sm">{t.sectionDescription}</p>
      </div>
      <TournamentAgenda rows={rows} slug={slug} />
    </section>
  );
}
