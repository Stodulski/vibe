import { FormField } from '@/shared/components/common/FormField';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import { ADJUSTMENT_REASONS, type AdjustmentReason } from '../lib/adjustmentReasons';

const t = ES_AR;

export function AdjustReasonField({
  value,
  onChange,
  error,
}: {
  value: AdjustmentReason;
  onChange: (reason: AdjustmentReason) => void;
  error?: string | undefined;
}) {
  return (
    <FormField label={t.products.adjustReasonField} htmlFor="adjust-reason" error={error}>
      <Select
        value={value}
        onValueChange={(v) => {
          onChange(v as AdjustmentReason);
        }}
      >
        <SelectTrigger id="adjust-reason">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {ADJUSTMENT_REASONS.map((reason) => (
            <SelectItem key={reason} value={reason}>
              {t.products.reasons[reason]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </FormField>
  );
}
