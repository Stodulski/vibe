import { formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicTournamentSignUpPath } from '../../lib/publicPaths';
import { AgendaRowShell, type AgendaMetaToken } from '../agenda/AgendaRowShell';
import { rowWhenTag, spotsNotice, type AgendaRow as AgendaRowData } from './agendaModel';

const t = BOOKING_MESSAGES.tournaments;

interface AgendaRowProps {
  row: AgendaRowData;
  slug: string;
  onViewDetail: (row: AgendaRowData) => void;
  /** Opens the waitlist of a full category. */
  onNotify: (row: AgendaRowData) => void;
}

/**
 * One category on the agenda: its day and time, its title under the sport's
 * label, one line of what it costs and how many places are left, and its
 * sign-up, or "Avisarme" when it is full.
 */
export function AgendaRow({ row, slug, onViewDetail, onNotify }: AgendaRowProps) {
  const { category } = row;
  const full = category.spots_left <= 0;
  const spots = full ? { text: BOOKING_MESSAGES.waitlist.full, last: true } : spotsNotice(category.spots_left);
  const meta: AgendaMetaToken[] = [
    { key: 'spots', text: spots.text, emphasis: spots.last },
    { key: 'price', text: formatPrice(category.price), unit: t.perPairShort },
  ];

  return (
    <AgendaRowShell
      date={row.date}
      label={row.label}
      title={row.title}
      time={rowWhenTag(row)}
      meta={meta}
      viewDetailLabel={t.viewDetail}
      onViewDetail={() => {
        onViewDetail(row);
      }}
      signUp={
        full
          ? {
              notifyLabel: BOOKING_MESSAGES.waitlist.notifyMe,
              onNotify: () => {
                onNotify(row);
              },
            }
          : { href: publicTournamentSignUpPath(slug, row.tournamentId, category.id), label: t.signUpCategory }
      }
    />
  );
}
