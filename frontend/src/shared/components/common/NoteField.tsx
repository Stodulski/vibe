import type { UseFormRegisterReturn } from 'react-hook-form';
import { Textarea } from '@/shared/components/ui/textarea';
import { ES_AR } from '@/shared/i18n/es_AR';
import { FormField } from './FormField';

const t = ES_AR;

/**
 * The "nota (opcional)" textarea shared by every cashbox form
 * (open/close/movement/void) and the product catalog's restock/adjust
 * dialogs.
 *
 * Moved here from `features/cash/components/` (pos-products-screen T5a):
 * `features/products` had duplicated this verbatim on the "features never
 * import from one another" reasoning, but the repo's rule for that case is to
 * move the shared piece to `shared/` instead — same move as
 * `shared/lib/paymentMethods.ts`.
 */
export function NoteField({
  id,
  label,
  register,
  error,
}: {
  id: string;
  label: string;
  register: UseFormRegisterReturn;
  error?: string | undefined;
}) {
  return (
    <FormField label={`${label} (${t.common.optional})`} htmlFor={id} error={error}>
      <Textarea id={id} {...register} maxLength={500} className="min-h-[40px] resize-none" />
    </FormField>
  );
}
