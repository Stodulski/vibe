import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SPORTS, COURT_TYPES } from './constants';
import type { UseFormSetValue } from 'react-hook-form';
import type { Sport, CourtType } from '@/shared/types/api.types';
import type { CreateCourtDto } from '@/features/courts';

const t = ES_AR;

export function CourtAttributeFields({ setValue }: { setValue: UseFormSetValue<CreateCourtDto> }) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 sm:gap-4">
      <div className="space-y-1.5">
        <Label htmlFor="court-sport" className="text-xs">
          {t.courts.sport}
        </Label>
        <Select
          defaultValue="padel"
          onValueChange={(v) => {
            setValue('sport', v as Sport);
          }}
        >
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
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="court-type" className="text-xs">
          {t.courts.courtType}
        </Label>
        <Select
          defaultValue="outdoor"
          onValueChange={(v) => {
            setValue('court_type', v as CourtType);
          }}
        >
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
      </div>
    </div>
  );
}
