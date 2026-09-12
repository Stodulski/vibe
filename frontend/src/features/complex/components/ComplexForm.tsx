import type { ReactNode } from 'react';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { UnsavedChangesDialog } from '@/shared/components/common/UnsavedChangesDialog';
import { useUnsavedChangesBlocker } from '@/shared/hooks/useUnsavedChangesBlocker';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import type { Complex } from '@/shared/types/api.types';
import { ComplexFormFields } from './complex-form/ComplexFormFields';
import { useComplexForm } from './complex-form/useComplexForm';

const t = ES_AR;

interface ComplexFormProps {
  complex?: Complex | undefined;
  onSuccess?: ((created?: Complex) => void) | undefined;
  /** Rendered beside the submit button — settings puts the delete menu here. */
  footerAction?: ReactNode;
  /**
   * Which fields to ask for. Defaults to all of them.
   *
   * Onboarding passes a subset: its first step needs the name, the public URL
   * and how to reach the place, and nothing else. The deposit, the
   * cancellation window and the twelve services all have defaults or are
   * optional, and a new owner cannot answer them before they have a court.
   *
   * A render prop rather than a `mode` string, so the caller composes the
   * groups it wants instead of this component branching on a flag — a boolean
   * here is the door `mode="admin"` walks through next.
   */
  fields?: (props: FieldsProps) => ReactNode;
}

/** What the `fields` render prop receives: the live form and its slug state. */
export interface FieldsProps {
  form: ReturnType<typeof useComplexForm>['form'];
  isEdit: boolean;
  slugValue: string | undefined;
  onSlugManualEdit: () => void;
  /** Derives the slug from the new name, unless it has been edited by hand. */
  onNameChange: (name: string) => void;
  /** The slug already saved, so the availability check skips it. */
  currentSlug?: string | undefined;
}

export function ComplexForm({ complex, onSuccess, footerAction, fields }: ComplexFormProps) {
  const { form, isEdit, slugValue, mutation, onSubmit, onNameChange, onSlugManualEdit } = useComplexForm({
    complex,
    onSuccess,
  });
  // FORM-11: a complex is a long form — name, public URL, address, deposit,
  // cancellation window, a dozen services. Leaving it half-filled used to cost
  // all of it silently.
  const blocker = useUnsavedChangesBlocker(form.formState.isDirty);

  return (
    // Capped at a readable measure. A field is sized for the answer it expects,
    // and an email box the width of the panel asks for an email that wide.
    <form onSubmit={submitHandler(form.handleSubmit, onSubmit)} className="mx-auto max-w-xl space-y-6">
      {(fields ?? ComplexFormFields)({
        form,
        isEdit,
        slugValue,
        onSlugManualEdit,
        onNameChange,
        currentSlug: complex?.slug,
      })}
      <SectionFooter
        submitLabel={isEdit ? t.complex.updateComplex : t.complex.createComplex}
        pending={mutation.isPending}
        submitSize="lg"
        align="start"
        extra={footerAction}
      />
      <UnsavedChangesDialog blocker={blocker} />
    </form>
  );
}
