import { NameSlugFields } from './NameSlugFields';
import { AddressField } from './AddressField';
import { ContactFields } from './ContactFields';
import { AmenitiesFieldset } from './AmenitiesFieldset';
import { DepositPercentageField } from '../deposit-config/DepositPercentageField';
import { CancellationHoursField } from '../deposit-config/CancellationHoursField';

import type { useComplexForm } from './useComplexForm';

type Form = ReturnType<typeof useComplexForm>['form'];

interface GroupProps {
  form: Form;
  isEdit: boolean;
  slugValue: string | undefined;
  onSlugManualEdit: () => void;
  /** Derives the slug from the new name, unless it has been edited by hand. */
  onNameChange: (name: string) => void;
  /** The slug this complex already owns, so it never collides with itself. */
  currentSlug?: string | undefined;
}

/**
 * Related fields, grouped by spacing alone.
 *
 * They carried uppercase headings — Identidad, Ubicación y contacto,
 * Servicios — and inside a single card describing a single complex those named
 * what the labels underneath already said. Proximity groups them on its own;
 * the headings were the reader being told a third time.
 */
function FieldGroup({ children }: { children: React.ReactNode }) {
  return <section className="space-y-4">{children}</section>;
}

/**
 * Who the complex is: its name and the address of its public page.
 *
 * Exported on its own because onboarding's first step needs THIS group and not
 * the others. The alternative was a `mode` flag inside the shared form, which
 * is the door `mode="admin"` walks through next: extend by composition, not by
 * branching what everyone shares.
 */
export function IdentityGroup({ form, isEdit, slugValue, onSlugManualEdit, onNameChange, currentSlug }: GroupProps) {
  return (
    <FieldGroup>
      <NameSlugFields
        register={form.register}
        errors={form.formState.errors}
        isEdit={isEdit}
        slugValue={slugValue}
        onSlugManualEdit={onSlugManualEdit}
        onNameChange={onNameChange}
        currentSlug={currentSlug}
        onSlugSuggestion={(slug) => {
          // Adopting a suggestion counts as choosing the slug by hand: the
          // name must stop overwriting it on the next keystroke.
          onSlugManualEdit();
          form.setValue('slug', slug, { shouldValidate: true });
        }}
      />
    </FieldGroup>
  );
}

/** Where the complex is and how to reach it. */
export function LocationGroup({ form }: { form: Form }) {
  const {
    register,
    setValue,
    watch,
    control,
    formState: { errors },
  } = form;
  return (
    <FieldGroup>
      <AddressField control={control} errors={errors} watch={watch} setValue={setValue} />
      <ContactFields register={register} control={control} errors={errors} />
    </FieldGroup>
  );
}

/**
 * What is charged up front and how long a client can change their mind.
 *
 * Not in onboarding: both have defaults the schema already supplies (30% and
 * 24h), and asking a brand new owner to price a cancellation policy before
 * they have a single court is asking a question they cannot answer yet.
 */
export function BillingGroup({ form }: { form: Form }) {
  return (
    <FieldGroup>
      {/* Side by side: two short related fields, each with exactly one line of
          helper text, so their inputs share a baseline. */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <DepositPercentageField register={form.register} error={form.formState.errors.deposit_percentage} />
        <CancellationHoursField register={form.register} error={form.formState.errors.cancellation_hours} />
      </div>
    </FieldGroup>
  );
}

/** The twelve services, also absent from onboarding — all of them optional. */
export function AmenitiesGroup({ form }: { form: Form }) {
  return (
    <FieldGroup>
      <AmenitiesFieldset control={form.control} />
    </FieldGroup>
  );
}

/**
 * Every group, in order — what the settings screen asks for.
 *
 * Onboarding composes a subset instead of passing this a flag.
 */
export function ComplexFormFields(props: GroupProps) {
  return (
    <>
      <IdentityGroup {...props} />
      <LocationGroup form={props.form} />
      <BillingGroup form={props.form} />
      <AmenitiesGroup form={props.form} />
    </>
  );
}
