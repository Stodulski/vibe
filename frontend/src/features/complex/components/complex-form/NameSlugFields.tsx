import type { ChangeEvent } from 'react';
import { useSlugAvailability, type SlugState } from './useSlugAvailability';
import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { RequiredMark } from '@/shared/components/common/RequiredMark';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CreateComplexDto } from '../../schemas/complex.schema';

const t = ES_AR;

interface NameSlugFieldsProps {
  register: UseFormRegister<CreateComplexDto>;
  errors: FieldErrors<CreateComplexDto>;
  isEdit: boolean;
  slugValue: string | undefined;
  onSlugManualEdit: () => void;
  /** Derives the slug from the new name, unless it has been edited by hand. */
  onNameChange: (name: string) => void;
  /** The slug this complex already owns — never a collision with itself. */
  currentSlug?: string | undefined;
  /** Adopts a server-suggested free variant into the form. */
  onSlugSuggestion: (slug: string) => void;
}

interface SlugSectionProps {
  register: UseFormRegister<CreateComplexDto>;
  errors: FieldErrors<CreateComplexDto>;
  isEdit: boolean;
  slugValue: string | undefined;
  onSlugManualEdit: () => void;
  availability: SlugState;
  onSlugSuggestion: (slug: string) => void;
  /** The slug this complex already owns — compared against the live value to show the change warning. */
  currentSlug?: string | undefined;
}

// The public address, as a plain field. It is derived from the name, so most
// owners never touch it, but it is also the ONE chance to choose it on
// create. On edit it stays editable too — changing it is a real, if risky,
// choice an owner sometimes needs to make (a rename, a typo) — but every link
// and printed QR already pointing at the old slug breaks, so a warning shows
// under the field while the typed value differs from the one already saved.
// It used to fold into a "Tu página pública: …" preview line with an
// "Editar" link; on a phone that line truncated the address it was there to
// show, so the field stays a field and the availability verdict sits right
// under it.
function SlugSection({
  register,
  errors,
  isEdit,
  slugValue,
  onSlugManualEdit,
  availability,
  onSlugSuggestion,
  currentSlug,
}: SlugSectionProps) {
  const showChangeWarning = isEdit && currentSlug !== undefined && slugValue !== undefined && slugValue !== currentSlug;

  return (
    <div className="space-y-2">
      <FormField
        label={
          <>
            {t.complex.slug} <RequiredMark />
          </>
        }
        htmlFor="slug"
        helpText={isEdit ? undefined : t.complex.slugHelp}
        error={errors.slug?.message}
      >
        <Input
          id="slug"
          // Capped once the column is wider than a slug needs; on a phone
          // the column is the cap, and a shorter box there reads as broken.
          className="sm:max-w-80"
          aria-invalid={!!errors.slug}
          {...register('slug', {
            onChange: () => {
              onSlugManualEdit();
            },
          })}
        />
      </FormField>
      {showChangeWarning && <p className="text-warning-text text-xs">{t.complex.slugChangeWarning}</p>}
      <AvailabilityNote state={availability} slugValue={slugValue} onUse={onSlugSuggestion} />
    </div>
  );
}

export function NameSlugFields({
  register,
  errors,
  isEdit,
  slugValue,
  onSlugManualEdit,
  onNameChange,
  currentSlug,
  onSlugSuggestion,
}: NameSlugFieldsProps) {
  // Runs on edit too: the slug is changeable there now, and a rename can
  // collide with another club exactly like a fresh one can. `currentSlug`
  // keeps a complex from reporting a collision with itself.
  const availability = useSlugAvailability(slugValue, currentSlug);

  return (
    <div className="space-y-4">
      <FormField
        label={
          <>
            {t.complex.name} <RequiredMark />
          </>
        }
        htmlFor="name"
        error={errors.name?.message}
      >
        <Input
          id="name"
          aria-invalid={!!errors.name}
          {...register('name', {
            onChange: (e: ChangeEvent<HTMLInputElement>) => {
              onNameChange(e.target.value);
            },
          })}
        />
      </FormField>

      <SlugSection
        register={register}
        errors={errors}
        isEdit={isEdit}
        slugValue={slugValue}
        onSlugManualEdit={onSlugManualEdit}
        availability={availability}
        onSlugSuggestion={onSlugSuggestion}
        currentSlug={currentSlug}
      />
    </div>
  );
}

/**
 * Free, taken, or nothing yet — said on the preview line.
 *
 * The verdict goes here and not only under the input, because the input is
 * where almost nobody looks: the slug is derived from the name and folded away
 * behind "Editar", so this line is the whole of what most owners ever see of
 * their public URL.
 */
function AvailabilityNote({
  state,
  slugValue,
  onUse,
}: {
  state: SlugState;
  slugValue: string | undefined;
  onUse: (slug: string) => void;
}) {
  if (state.status === 'free' && slugValue) {
    return <p className="text-success-text text-xs">{t.complex.slugAvailable}</p>;
  }
  if (state.status !== 'taken') return null;

  return (
    <p className="text-error-text text-xs">
      {t.validation.server.slugTaken}
      {state.suggestion && (
        <>
          {' '}
          <button
            type="button"
            onClick={() => {
              onUse(state.suggestion ?? '');
            }}
            className="focus-self font-medium underline"
          >
            {t.complex.slugUseSuggestion} {state.suggestion}
          </button>
        </>
      )}
    </p>
  );
}
