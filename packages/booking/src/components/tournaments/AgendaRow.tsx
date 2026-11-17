import { formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicTournamentSignUpPath } from '../../lib/publicPaths';
import { AgendaRowShell, type AgendaMetaToken } from '../agenda/AgendaRowShell';
import { spotsNotice, type AgendaRow as AgendaRowData } from './agendaModel';

const t = BOOKING_MESSAGES.tournaments;

interface AgendaRowProps {
  row: AgendaRowData;
  slug: string;
  onViewDetail: (row: AgendaRowData) => void;
}

/**
 * One category on the agenda: its day and time, its title under the sport's
 * label, one line of what it costs and how many places are left, and its
 * sign-up.
 */
export function AgendaRow({ row, slug, onViewDetail }: AgendaRowProps) {
  const { category } = row;
  const spots = spotsNotice(category.spots_left);
  const meta: AgendaMetaToken[] = [
    { key: 'spots', text: spots.text, emphasis: spots.last },
    { key: 'price', text: `${formatPrice(category.price)} ${t.perPair}` },
  ];

  return (
    <AgendaRowShell
      date={row.date}
      label={row.label}
      title={row.title}
      time={row.time === null ? undefined : `${row.time} ${t.hoursSuffix}`}
      meta={meta}
      viewDetailLabel={t.viewDetail}
      onViewDetail={() => {
        onViewDetail(row);
      }}
      signUp={{ href: publicTournamentSignUpPath(slug, row.tournamentId, category.id), label: t.signUpCategory }}
    />
  );
}
