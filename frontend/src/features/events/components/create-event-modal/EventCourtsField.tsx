import { type UseFormReturn } from 'react-hook-form';
import { courtFactsLine } from '@vibe/booking/court-facts';
import { FieldRequirement } from '@/shared/components/common/FieldRequirement';
import { Checkbox } from '@/shared/components/ui/checkbox';
import { Label } from '@/shared/components/ui/label';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtWithPrices } from '@/shared/types/api.types';
import type { EventFormValues } from '../../schemas/eventForm.schema';

const t = ES_AR.events.form;

/**
 * Which courts the event holds: all of the complex's active ones by default,
 * or a chosen subset once "Todas las canchas" is turned off. Each court shows its
 * roof, surface and lighting under its name, as players see them when they sign up.
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
      <legend className="text-text-secondary text-xs font-medium">
        {t.courts}
        <FieldRequirement required />
      </legend>
      <p className="text-text-tertiary text-xs">{t.courtsHint}</p>
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
            <CourtOption
              key={court.id}
              court={court}
              checked={courtIds.includes(court.id)}
              onCheckedChange={(checked) => {
                toggleCourt(court.id, checked);
              }}
            />
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

/** One court to tick: its name, and under it the roof, surface and lighting that describe the checkbox. */
function CourtOption({
  court,
  checked,
  onCheckedChange,
}: {
  court: CourtWithPrices;
  checked: boolean;
  onCheckedChange: (checked: boolean) => void;
}) {
  const facts = courtFactsLine(court);
  const id = `event-court-${court.id}`;
  return (
    <div className="flex min-w-0 items-start gap-2.5">
      <Checkbox
        id={id}
        className="mt-0.5"
        aria-describedby={facts ? `${id}-facts` : undefined}
        checked={checked}
        onCheckedChange={(value) => {
          onCheckedChange(value === true);
        }}
      />
      <div className="min-w-0">
        <Label htmlFor={id} className="block truncate text-sm">
          {court.name}
        </Label>
        {facts && (
          <p id={`${id}-facts`} className="text-text-tertiary truncate text-xs" title={facts}>
            {facts}
          </p>
        )}
      </div>
    </div>
  );
}
