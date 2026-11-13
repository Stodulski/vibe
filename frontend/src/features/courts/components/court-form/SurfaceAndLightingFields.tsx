import type { Control, FieldErrors } from 'react-hook-form';
import { Controller, useWatch } from 'react-hook-form';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { Switch } from '@/shared/components/ui/switch';
import { ES_AR } from '@/shared/i18n/es_AR';
import { surfacesForSport } from '../../lib/surfaces';
import type { CreateCourtDto } from '../../schemas/courts.schema';

const t = ES_AR;

// Radix Select reserves '' for "clear the selection and show the placeholder",
// so the "Sin especificar" item needs a value of its own; it maps back to ''.
const UNSPECIFIED = 'none';

interface SurfaceAndLightingFieldsProps {
  control: Control<CreateCourtDto>;
  errors: FieldErrors<CreateCourtDto>;
}

/**
 * The floor and the lights. The surface list follows the sport chosen above it;
 * the reset when the sport changes lives in `SportAndTypeFields`, next to the
 * control that causes it.
 */
export function SurfaceAndLightingFields({ control, errors }: SurfaceAndLightingFieldsProps) {
  const sport = useWatch({ control, name: 'sport' });
  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <Label htmlFor="court-surface">{t.courts.surface}</Label>
        <Controller
          name="surface"
          control={control}
          render={({ field }) => (
            <Select
              value={field.value === undefined || field.value === '' ? UNSPECIFIED : field.value}
              onValueChange={(v) => {
                field.onChange(v === UNSPECIFIED ? '' : v);
              }}
            >
              <SelectTrigger id="court-surface" aria-invalid={!!errors.surface}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={UNSPECIFIED}>{t.courts.surfaceUnspecified}</SelectItem>
                {surfacesForSport(sport).map((s) => (
                  <SelectItem key={s} value={s}>
                    {t.courts.surfaces[s]}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        />
        {errors.surface && <p className="text-error-text text-sm">{errors.surface.message}</p>}
      </div>

      <div className="flex items-center justify-between gap-3">
        <Label htmlFor="court-lighting">{t.courts.hasLighting}</Label>
        <Controller
          name="has_lighting"
          control={control}
          render={({ field }) => (
            <Switch
              id="court-lighting"
              checked={field.value ?? false}
              onCheckedChange={field.onChange}
              aria-label={t.courts.hasLighting}
            />
          )}
        />
      </div>
    </div>
  );
}
