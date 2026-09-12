import type { Control, FieldErrors } from 'react-hook-form';
import { Controller } from 'react-hook-form';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SPORTS, COURT_TYPES } from './options';
import type { CreateCourtDto } from '../../schemas/courts.schema';

const t = ES_AR;

interface SportAndTypeFieldsProps {
  control: Control<CreateCourtDto>;
  errors: FieldErrors<CreateCourtDto>;
}

export function SportAndTypeFields({ control, errors }: SportAndTypeFieldsProps) {
  return (
    // One field per row, at every width. Two dropdowns sharing a row read as
    // one compound question — "deporte y tipo" — when they are two separate
    // decisions, and a form is easiest to answer when each row asks for
    // exactly one thing.
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="court-sport">{t.courts.sport}</Label>
        <Controller
          name="sport"
          control={control}
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger id="court-sport">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SPORTS.map((s) => (
                  <SelectItem key={s.value} value={s.value}>
                    {s.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        {errors.sport && <p className="text-error-text text-sm">{errors.sport.message}</p>}
      </div>

      <div className="space-y-2">
        <Label htmlFor="court-type">{t.courts.courtType}</Label>
        <Controller
          name="court_type"
          control={control}
          render={({ field }) => (
            <Select value={field.value} onValueChange={field.onChange}>
              <SelectTrigger id="court-type">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {COURT_TYPES.map((ct) => (
                  <SelectItem key={ct.value} value={ct.value}>
                    {ct.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        {errors.court_type && <p className="text-error-text text-sm">{errors.court_type.message}</p>}
      </div>
    </div>
  );
}
