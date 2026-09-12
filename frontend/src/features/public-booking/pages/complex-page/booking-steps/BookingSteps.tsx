import { ES_AR } from '@/shared/i18n/es_AR';
import { DURATION_OPTIONS } from '@/features/public-booking';
import type { DurationMinutes, Sport } from '@/shared/types/api.types';
import { StepChoice } from './StepChoice';
import { StepBreadcrumbs, type Crumb } from '@/shared/components/common/StepBreadcrumbs';
import { useBookingSteps, type StepId } from './useBookingSteps';

const t = ES_AR;

interface BookingStepsProps {
  availableSports: Sport[];
  sportFilter: Sport | null;
  onSportFilterChange: (sport: Sport | null) => void;
  duration: DurationMinutes;
  /** `null` until answered — what the duration `StepChoice` marks as selected. */
  selectedDuration: DurationMinutes | null;
  onDurationChange: (duration: DurationMinutes) => void;
  /**
   * The hour whose court question is open, if one is. Shown as a crumb in
   * the same row as the sport and duration, so the three answers read as one
   * line; its pencil returns to the hours.
   */
  timeAnswer: string | null;
  onEditTime: () => void;
  /** Questions the URL already answered, so they open as done. */
  answeredFromUrl?: { sport: boolean; duration: boolean };
  /** The hours, rendered once every question has an answer. */
  children: React.ReactNode;
}

type CrumbId = StepId | 'time';

/** The answered steps as crumbs, plus the open hour when a court is being asked. */
function buildCrumbs(
  answered: StepId[],
  sportFilter: Sport | null,
  duration: DurationMinutes,
  timeAnswer: string | null,
): Crumb<CrumbId>[] {
  const crumbs: Crumb<CrumbId>[] = answered.map((id) =>
    id === 'sport'
      ? {
          id,
          label: t.publicBooking.stepSportLabel,
          answer: sportFilter ? t.courts.sportTypes[sportFilter] : '',
        }
      : {
          id,
          label: t.publicBooking.stepDurationLabel,
          answer: `${String(duration)} ${t.publicBooking.minutesShort}`,
        },
  );
  if (timeAnswer !== null) {
    crumbs.push({ id: 'time', label: t.publicBooking.schedule, answer: timeAnswer });
  }
  return crumbs;
}

/**
 * The questions that come before the hours, and the hours themselves.
 *
 * One question on screen at a time, each answered with a tap that moves on.
 * The point is not fewer taps — it is more: a grid that mixes every court
 * together lets someone tap 20:00 and only then learn that the single court
 * left is not what they wanted. Asking what changes the inventory first means
 * the hours on screen are already true for what was asked.
 *
 * The date is not one of these. Every step here narrows the inventory inside
 * a day; the date chooses which day is being narrowed, and it is what people
 * sweep across most ("and tomorrow?"). It stays above, always visible.
 */
export function BookingSteps({
  availableSports,
  sportFilter,
  onSportFilterChange,
  duration,
  selectedDuration,
  onDurationChange,
  timeAnswer,
  onEditTime,
  answeredFromUrl,
  children,
}: BookingStepsProps) {
  const steps = useBookingSteps({ availableSports, duration, initiallyAnswered: answeredFromUrl });

  // A question asked on the way in owns the screen; one re-opened from a
  // breadcrumb sits above the hours it is about to change.
  const question =
    steps.activeStep === 'sport' ? (
      <StepChoice
        question={t.publicBooking.stepSportQuestion}
        choices={availableSports.map((sport) => ({
          value: sport,
          label: t.courts.sportTypes[sport],
        }))}
        selected={sportFilter}
        onChoose={(value) => {
          onSportFilterChange(value as Sport);
          steps.answer('sport');
        }}
      />
    ) : steps.activeStep === 'duration' ? (
      <StepChoice
        question={t.publicBooking.stepDurationQuestion}
        choices={DURATION_OPTIONS.map((option) => ({
          value: String(option),
          label: `${String(option)} ${t.publicBooking.minutesShort}`,
        }))}
        // `selectedDuration`, not `duration`: `duration` is defaulted for the
        // availability fetch (see `useComplexPageState`) and would otherwise
        // paint 90 min as chosen before the visitor has answered anything.
        selected={selectedDuration === null ? null : String(selectedDuration)}
        onChoose={(value) => {
          onDurationChange(Number(value) as DurationMinutes);
          steps.answer('duration');
        }}
      />
    ) : null;

  if (question && steps.isFirstPass) return question;

  // A question re-opened while the court question was open goes back to
  // being just the question: the hours it would sit above belong to the
  // answer that is about to change, and the court cards even more so.
  // Answering it drops the open hour (see `useComplexPageState`), and the
  // new grid appears underneath as usual.
  const showsHours = !question || timeAnswer === null;

  return (
    <div className="space-y-5">
      <StepBreadcrumbs
        crumbs={buildCrumbs(steps.answered, sportFilter, duration, question ? null : timeAnswer)}
        onEdit={(id) => {
          if (id === 'time') onEditTime();
          else steps.edit(id);
        }}
      />
      {question}
      {showsHours && children}
    </div>
  );
}
