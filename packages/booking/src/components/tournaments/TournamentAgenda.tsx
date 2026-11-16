import { useState } from 'react';
import { cn } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { modalityLabel } from '../../lib/traitLabels';
import { AgendaDetailDialog } from './AgendaDetailDialog';
import { AgendaRow } from './AgendaRow';
import {
  dayChips,
  filterAgenda,
  type AgendaFilter,
  type AgendaRow as AgendaRowData,
  type ModalityFilter,
} from './agendaModel';

const t = BOOKING_MESSAGES.tournaments;

/** The modality filters, in the order the agenda offers them. */
const MODALITY_FILTERS: { value: ModalityFilter; label: string }[] = [
  { value: 'all', label: t.filterAll },
  { value: 'women', label: modalityLabel('women', 'padel') },
  { value: 'men', label: modalityLabel('men', 'padel') },
  { value: 'mixed', label: modalityLabel('mixed', 'padel') },
];

interface ChipProps {
  pressed: boolean;
  onClick: () => void;
  children: string;
}

/** A filter chip, styled like the day and option buttons of the booking flow. */
function Chip({ pressed, onClick, children }: ChipProps) {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      onClick={onClick}
      className={cn(
        'press-scale h-10 cursor-pointer rounded-full border px-4 text-sm font-medium whitespace-nowrap transition-colors duration-200',
        pressed
          ? 'border-primary-500 bg-primary-500/10 text-primary-400'
          : 'border-border-interactive bg-bg-subtle text-text-secondary hover:border-border-interactive-hover hover:bg-bg-highlight',
      )}
    >
      {children}
    </button>
  );
}

interface TournamentAgendaProps {
  /** The rows to list: the categories open for sign-up with a place left (see `agendaRows`). */
  rows: AgendaRowData[];
  slug: string;
}

interface AgendaFiltersProps {
  rows: AgendaRowData[];
  filter: AgendaFilter;
  onChange: (filter: AgendaFilter) => void;
}

/** The modality chips and the day chips above the list. Each one replaces its own part of the filter. */
function AgendaFilters({ rows, filter, onChange }: AgendaFiltersProps) {
  return (
    <>
      <div role="group" aria-label={t.filterModality} className="flex flex-wrap gap-2">
        {MODALITY_FILTERS.map((option) => (
          <Chip
            key={option.value}
            pressed={filter.modality === option.value}
            onClick={() => {
              onChange({ ...filter, modality: option.value });
            }}
          >
            {option.label}
          </Chip>
        ))}
      </div>

      <div role="group" aria-label={t.filterDay} className="flex flex-wrap gap-2">
        <Chip
          pressed={filter.day === null}
          onClick={() => {
            onChange({ ...filter, day: null });
          }}
        >
          {t.filterAllDays}
        </Chip>
        {dayChips(rows).map((chip) => (
          <Chip
            key={chip.date}
            pressed={filter.day === chip.date}
            onClick={() => {
              onChange({ ...filter, day: chip.date });
            }}
          >
            {chip.label}
          </Chip>
        ))}
      </div>
    </>
  );
}

/**
 * The public agenda of the complex's tournaments: one row per open category,
 * sorted by day, with a modality filter and a day filter above it. Its detail
 * opens in a dialog, and each row signs up straight to its category.
 */
export function TournamentAgenda({ rows, slug }: TournamentAgendaProps) {
  const [filter, setFilter] = useState<AgendaFilter>({ modality: 'all', day: null });
  const [detail, setDetail] = useState<AgendaRowData | null>(null);
  const visible = filterAgenda(rows, filter);

  return (
    <div className="flex flex-col gap-4">
      <p className="text-text-secondary text-sm">{t.onlyWithSpots}</p>

      <AgendaFilters rows={rows} filter={filter} onChange={setFilter} />

      {visible.length === 0 ? (
        <p className="text-text-secondary text-sm">{t.noAgendaRows}</p>
      ) : (
        <ul className="divide-border-subtle flex flex-col divide-y">
          {visible.map((row) => (
            <li key={row.category.id}>
              <AgendaRow row={row} slug={slug} onViewDetail={setDetail} />
            </li>
          ))}
        </ul>
      )}

      <AgendaDetailDialog
        row={detail}
        slug={slug}
        onClose={() => {
          setDetail(null);
        }}
      />
    </div>
  );
}
