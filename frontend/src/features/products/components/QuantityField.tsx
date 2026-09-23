import { FormField } from '@/shared/components/common/FormField';
import { Input } from '@/shared/components/ui/input';

interface QuantityFieldProps {
  id: string;
  label: string;
  value: number | undefined;
  onChange: (value: number | undefined) => void;
  error?: string | undefined;
  min?: number;
  placeholder?: string | undefined;
  helpText?: string | undefined;
}

/**
 * A plain integer-count input — shared by the low-stock threshold, the
 * restock quantity and the adjustment's "cantidad contada" (one helper per
 * label rule instead of three near-duplicates, per the cash review's own
 * "controlled selects driven by the form value" lesson applied to number
 * inputs here). Not `MoneyPesosField`: this is a count, not a peso amount —
 * no banknote icon, no money-specific caps.
 *
 * Same NaN-from-a-blank-input handling as `MoneyPesosField`: `valueAsNumber`
 * reports an empty field as `NaN`, turned into `undefined` here so the Zod
 * schemas report "ingresá una cantidad" instead of a value silently reset to 0.
 */
export function QuantityField({
  id,
  label,
  value,
  onChange,
  error,
  min = 0,
  placeholder,
  helpText,
}: QuantityFieldProps) {
  return (
    <FormField label={label} htmlFor={id} error={error} helpText={helpText}>
      <Input
        id={id}
        type="number"
        inputMode="numeric"
        min={min}
        value={value === undefined || Number.isNaN(value) ? '' : value}
        placeholder={placeholder}
        onChange={(e) => {
          const raw = e.target.valueAsNumber;
          onChange(Number.isNaN(raw) ? undefined : raw);
        }}
      />
    </FormField>
  );
}
