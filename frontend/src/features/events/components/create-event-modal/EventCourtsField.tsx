import { type UseFormReturn } from 'react-hook-form';
import { Checkbox } from '@/shared/components/ui/checkbox';
import { Label } from '@/shared/components/ui/label';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtWithPrices } from '@/shared/types/api.types';
import type { EventFormValues } from '../../schemas/eventForm.schema';

const t = ES_AR.events.form;

/**
 * Which courts the event holds: all of the complex's active ones by default,
 * or a chosen subset once "Todas las canchas" is turned off.
 */
export function EventCourtsField({
  form,
  courts,
}: {
  form: UseFormReturn<EventFormValues>;
  courts: CourtWithPrices[];
}) {
  const { watch, setValue, formState } = form;
  const [allCourts, courtIds] = watch(['allCourts', 'courtIds']);
  const activeCourts = courts.filter((court) => court.is_active);
  const error = formState.errors.courtIds?.message;

  const toggleCourt = (courtId: string, checked: boolean) => {
    const next = checked ? [...courtIds, courtId] : courtIds.filter((id) => id !== courtId);
    setValue('courtIds', next, { shouldValidate: true });
  };

  return (
    <fieldset className="space-y-2.5">
      <legend className="text-text-primary text-sm font-medium">{t.courts}</legend>
      <div className="flex items-center gap-2.5">
        <Checkbox
          id="event-all-courts"
          checked={allCourts}
          onCheckedChange={(checked) => {
            setValue('allCourts', checked === true, { shouldValidate: true });
          }}
        />
        <Label htmlFor="event-all-courts" className="text-sm">
          {t.allCourts}
        </Label>
      </div>
      {!allCourts && (
        <div className="border-border-subtle grid grid-cols-1 gap-2.5 rounded-xl border p-3 sm:grid-cols-2">
          {activeCourts.map((court) => (
            <div key={court.id} className="flex min-w-0 items-center gap-2.5">
              <Checkbox
                id={`event-court-${court.id}`}
                checked={courtIds.includes(court.id)}
                onCheckedChange={(checked) => {
                  toggleCourt(court.id, checked === true);
                }}
              />
              <Label htmlFor={`event-court-${court.id}`} className="min-w-0 truncate text-sm">
                {court.name}
              </Label>
            </div>
          ))}
        </div>
      )}
      {error && (
        <p role="alert" className="text-error-text text-xs">
          {error}
        </p>
      )}
    </fieldset>
  );
}
