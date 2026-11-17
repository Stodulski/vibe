import { formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicTournamentSignUpPath } from '../../lib/publicPaths';
import type { AgendaDetail } from '../agenda/AgendaDetailDialog';
import { categoryWhen, formatDetailLines, type AgendaRow } from './agendaModel';

const t = BOOKING_MESSAGES.tournaments;

/**
 * The detail of a tournament category: when it starts, what a pair pays, its
 * prizes when it has them, and its format, with the sign-up to it.
 */
export function tournamentDetail(row: AgendaRow, slug: string): AgendaDetail {
  const { category } = row;
  return {
    title: row.title,
    when: categoryWhen(row),
    facts: [
      { term: t.inscriptionLabel, value: `${formatPrice(category.price)} ${t.perPair}` },
      ...(category.prizes ? [{ term: t.prizesLabel, value: category.prizes }] : []),
      { term: t.formatLabel, value: formatDetailLines(category) },
    ],
    signUp: { href: publicTournamentSignUpPath(slug, row.tournamentId, category.id), label: t.signUpCategory },
  };
}
