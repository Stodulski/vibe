import type { ReactNode } from 'react';
import { FormField } from '@/shared/components/common/FormField';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { generateTimeSlots } from '@/shared/lib/time';

const WHOLE_DAY = generateTimeSlots();

/**
 * A labelled picker for one time of day.
 *
 * `options` is the list to offer; it defaults to every half hour of the day,
 * which is the opt-out for a caller that really means 24h (a complex setting its
 * own opening hours). A picker for something that happens inside a complex
 * passes `timeOptionsForOpeningHours(schedules, [value])` so nobody can choose
 * an hour the complex is closed. `shared/` cannot read a complex's schedule
 * itself (it must not import a feature), hence the prop rather than a default.
 */
export function TimeSelectField({
  id,
  label,
  value,
  error,
  onChange,
  options = WHOLE_DAY,
  helpText,
}: {
  id: string;
  label: ReactNode;
  value: string;
  error?: string | undefined;
  onChange: (time: string) => void;
  options?: readonly string[];
  helpText?: ReactNode;
}) {
  return (
    <FormField label={label} htmlFor={id} error={error} helpText={helpText}>
      <Select value={value} onValueChange={onChange}>
        <SelectTrigger id={id}>
          <SelectValue placeholder="--:--" />
        </SelectTrigger>
        <SelectContent>
          {options.map((time) => (
            <SelectItem key={time} value={time}>
              {time}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}
