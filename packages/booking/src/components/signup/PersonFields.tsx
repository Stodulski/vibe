import {
  Controller,
  get,
  type Control,
  type FieldErrors,
  type FieldValues,
  type Path,
  type UseFormRegister,
} from 'react-hook-form';
import { FieldRequirement, FormField, Input, PhoneInput } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

export interface PersonFieldLabels {
  firstName: string;
  lastName: string;
  phone: string;
  phoneHelp?: string;
}

interface PersonFieldsProps<T extends FieldValues> {
  register: UseFormRegister<T>;
  control: Control<T>;
  errors: FieldErrors<T>;
  /** Where the person's fields live in the form: `''` for the form's own, `'partner.'` for a nested person. */
  namePrefix?: string;
  /** Keeps the element ids of two people on one page apart (`event_`, `pair_partner_`). */
  idPrefix: string;
  labels?: PersonFieldLabels;
  /** Email is required; a form that does not ask for it leaves it out. */
  withEmail?: boolean;
}

const DEFAULT_LABELS: PersonFieldLabels = {
  firstName: t.publicBooking.firstName,
  lastName: t.publicBooking.lastName,
  phone: t.publicBooking.phone,
  phoneHelp: t.publicBooking.phoneHelper,
};

/** How one field of the person is named in the form, in the DOM and in the errors. */
interface FieldAccess<T extends FieldValues> {
  register: UseFormRegister<T>;
  control: Control<T>;
  path: (field: string) => Path<T>;
  id: (field: string) => string;
  errorOf: (field: string) => string | undefined;
  /** The person is not the one the browser knows: no autofill of the signed-in person's name. */
  nested: boolean;
}

function NameFields<T extends FieldValues>({ access, labels }: { access: FieldAccess<T>; labels: PersonFieldLabels }) {
  const { register, path, id, errorOf, nested } = access;
  return (
    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
      <FormField
        label={
          <>
            {labels.firstName}
            <FieldRequirement required />
          </>
        }
        htmlFor={id('first_name')}
        error={errorOf('first_name')}
      >
        <Input
          id={id('first_name')}
          autoComplete={nested ? 'off' : 'given-name'}
          placeholder={t.placeholders.firstName}
          className="h-12 text-base"
          aria-invalid={!!errorOf('first_name')}
          aria-describedby={errorOf('first_name') ? `${id('first_name')}-error` : undefined}
          {...register(path('first_name'))}
        />
      </FormField>
      <FormField
        label={
          <>
            {labels.lastName}
            <FieldRequirement required />
          </>
        }
        htmlFor={id('last_name')}
        error={errorOf('last_name')}
      >
        <Input
          id={id('last_name')}
          autoComplete={nested ? 'off' : 'family-name'}
          placeholder={t.placeholders.lastName}
          className="h-12 text-base"
          aria-invalid={!!errorOf('last_name')}
          aria-describedby={errorOf('last_name') ? `${id('last_name')}-error` : undefined}
          {...register(path('last_name'))}
        />
      </FormField>
    </div>
  );
}

function PhoneField<T extends FieldValues>({ access, labels }: { access: FieldAccess<T>; labels: PersonFieldLabels }) {
  const { control, path, id, errorOf } = access;
  return (
    <FormField
      label={
        <>
          {labels.phone}
          <FieldRequirement required />
        </>
      }
      htmlFor={id('phone')}
      {...(labels.phoneHelp ? { helpText: labels.phoneHelp } : {})}
      error={errorOf('phone')}
    >
      <Controller
        name={path('phone')}
        control={control}
        render={({ field }) => (
          <PhoneInput
            id={id('phone')}
            value={field.value}
            onChange={field.onChange}
            onBlur={field.onBlur}
            placeholder="11 2345 6789"
            inputClassName="h-12 text-base"
            aria-invalid={!!errorOf('phone')}
            aria-describedby={errorOf('phone') ? `${id('phone')}-error` : undefined}
          />
        )}
      />
    </FormField>
  );
}

function EmailField<T extends FieldValues>({ access }: { access: FieldAccess<T> }) {
  const { register, path, id, errorOf } = access;
  return (
    <FormField
      label={
        <>
          {t.publicBooking.email}
          <FieldRequirement required />
        </>
      }
      htmlFor={id('email')}
      error={errorOf('email')}
    >
      <Input
        id={id('email')}
        type="email"
        inputMode="email"
        autoComplete="email"
        placeholder={t.publicBooking.emailPlaceholder}
        className="h-12 text-base"
        aria-invalid={!!errorOf('email')}
        aria-describedby={errorOf('email') ? `${id('email')}-error` : undefined}
        {...register(path('email'))}
      />
    </FormField>
  );
}

/**
 * One person's sign-up fields: name and phone required, and the email too where
 * the form asks for it. The same chrome as the booking form, shared by every
 * public sign-up (an event's person, a tournament's payer and partner).
 */
export function PersonFields<T extends FieldValues>({
  register,
  control,
  errors,
  namePrefix = '',
  idPrefix,
  labels = DEFAULT_LABELS,
  withEmail = false,
}: PersonFieldsProps<T>) {
  const path = (field: string) => `${namePrefix}${field}` as Path<T>;
  const access: FieldAccess<T> = {
    register,
    control,
    path,
    id: (field) => `${idPrefix}${field}`,
    errorOf: (field) => (get(errors, path(field)) as { message?: string } | undefined)?.message,
    nested: namePrefix !== '',
  };
  return (
    <>
      <NameFields access={access} labels={labels} />
      <PhoneField access={access} labels={labels} />
      {withEmail && <EmailField access={access} />}
    </>
  );
}
