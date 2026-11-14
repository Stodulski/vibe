import { useId } from 'react';
import type { ReactNode } from 'react';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { Button } from '@/shared/components/ui/button';
import { keepTogether } from '@/shared/lib/keepTogether';
import { formatSeriesDay, weekdayOf } from '../../lib/seriesDates';
import type { BookingSeriesConflict, CourtAlternative } from '@/shared/types/api.types';

const t = ES_AR;
const s = t.bookings.series;

/** What holds a date that collides, as a short tag: the holder when the server says, else the reason. */
function conflictTag(conflict: BookingSeriesConflict): string {
  if (conflict.reason === 'overlap' && conflict.held_by) return s.conflictTags[conflict.held_by];
  return s.conflictTags[conflict.reason];
}

/** The line under the title: the two ways out, led by the one that fits the courts on offer. */
function conflictsHint(court: string, count: number, alternatives: CourtAlternative[]): string {
  if (alternatives.length === 0) return s.conflictsHintNone(court, count);
  const anyFree = alternatives.some((alt) => alt.conflicts === 0);
  return anyFree ? s.conflictsHintFree(court, count) : s.conflictsHintFewer(court, count);
}

/** One of the two ways out: a titled block, named by its title for assistive technology. */
function Option({ title, children }: { title: string; children: ReactNode }) {
  const id = useId();
  return (
    <section aria-labelledby={id} className="space-y-2">
      <h4 id={id} className="text-text-primary text-sm font-medium">
        {title}
      </h4>
      {children}
    </section>
  );
}

/** The other courts that fit the series better, each with a real button to switch to it. */
function CourtAlternatives({
  alternatives,
  weekday,
  pending,
  onUseCourt,
}: {
  alternatives: CourtAlternative[];
  weekday: number;
  pending: boolean;
  onUseCourt: (courtId: string) => void;
}) {
  const plural = s.weekdaysPlural[weekday] ?? '';
  const singular = s.weekdaysSingular[weekday] ?? '';
  return (
    <Option title={s.changeCourtTitle}>
      <ul className="space-y-2">
        {alternatives.map((alt) => (
          <li
            key={alt.court_id}
            data-testid="court-alternative"
            className="border-border flex flex-wrap items-center justify-between gap-x-3 gap-y-2 rounded-lg border p-3"
          >
            <div className="min-w-0 text-sm">
              <p className="text-text-primary font-medium">{keepTogether(alt.court_name)}</p>
              <p className="text-text-tertiary">
                {alt.conflicts === 0
                  ? s.alternativeAllFree(plural)
                  : s.alternativeBusy(alt.conflicts, plural, singular)}
              </p>
            </div>
            <Button
              type="button"
              variant="secondary"
              size="sm"
              disabled={pending}
              onClick={() => {
                onUseCourt(alt.court_id);
              }}
            >
              {s.useCourt(keepTogether(alt.court_name))}
            </Button>
          </li>
        ))}
      </ul>
    </Option>
  );
}

/**
 * What the create-series call refused, as the two ways out: switch to another
 * court, or stay and create the series without the colliding dates. A series
 * with no end only lists the year ahead, which the panel says.
 */
export function SeriesConflictsPanel({
  courtName,
  conflicts,
  endless,
  pending,
  alternatives,
  onUseCourt,
  onSkip,
  onBack,
}: {
  /** The court the series was refused on. */
  courtName: string;
  conflicts: BookingSeriesConflict[];
  /** The series has no end, so only the year ahead was checked. */
  endless: boolean;
  pending: boolean;
  /** Other courts the server says fit the series better, best first. */
  alternatives: CourtAlternative[];
  onUseCourt: (courtId: string) => void;
  onSkip: () => void;
  onBack: () => void;
}) {
  const court = keepTogether(courtName);
  const count = conflicts.length;
  const firstDate = conflicts[0]?.date;
  const weekday = firstDate ? weekdayOf(firstDate) : 0;
  const hint = conflictsHint(court, count, alternatives);

  return (
    // On a phone the dialog fills the screen and its form keeps the footer at the
    // bottom (see `DialogContent`); this panel is not a form, so it asks for the same.
    <div className="flex flex-1 flex-col gap-5 sm:flex-none">
      <div className="space-y-1">
        <h3 className="text-text-primary text-base font-semibold">
          {s.conflictsTitle(court, count, s.weekdaysPlural[weekday] ?? '', s.weekdaysSingular[weekday] ?? '')}
        </h3>
        <p className="text-text-secondary text-sm">{hint}</p>
      </div>
      {alternatives.length > 0 && (
        <CourtAlternatives alternatives={alternatives} weekday={weekday} pending={pending} onUseCourt={onUseCourt} />
      )}
      <Option title={s.stayOnCourtTitle(court)}>
        <p className="text-text-secondary text-sm">{s.skippedDaysLabel}</p>
        <ul className="max-h-60 space-y-1.5 overflow-y-auto text-sm">
          {conflicts.map((conflict) => (
            <li key={conflict.date} className="flex flex-wrap items-baseline justify-between gap-x-3">
              <span className="text-text-primary">{formatSeriesDay(conflict.date)}</span>
              <span className="text-text-tertiary text-xs">{conflictTag(conflict)}</span>
            </li>
          ))}
        </ul>
        {endless && <p className="text-text-tertiary text-xs">{s.conflictsEndless}</p>}
      </Option>
      <SectionFooter
        className="mt-auto sm:mt-0"
        onCancel={onBack}
        cancelLabel={s.conflictsBack}
        cancelDisabled={pending}
        submitLabel={s.skipConflicts(count)}
        pending={pending}
        onSubmit={onSkip}
      />
    </div>
  );
}
