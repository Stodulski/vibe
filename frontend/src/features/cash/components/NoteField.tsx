import type { UseFormRegisterReturn } from 'react-hook-form';
import { FormField } from '@/shared/components/common/FormField';
import { Textarea } from '@/shared/components/ui/textarea';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/** The "nota (opcional)" textarea shared by every cashbox form (open/close/movement/void). */
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
