import { FieldRequirement } from '@/shared/components/common/FieldRequirement';

/** A field's label with its "(requerido)" or "(opcional)" mark, the way the booking form writes it. */
export function FieldLabel({ text, required }: { text: string; required: boolean }) {
  return (
    <>
      {text}
      <FieldRequirement required={required} />
    </>
  );
}
