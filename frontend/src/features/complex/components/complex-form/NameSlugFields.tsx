import { useState, type ChangeEvent } from 'react';
import { useSlugAvailability, type SlugState } from './useSlugAvailability';
import type { FieldErrors, UseFormRegister } from 'react-hook-form';
import { Input } from '@/shared/components/ui/input';
import { FormField } from '@/shared/components/common/FormField';
import { RequiredMark } from '@/shared/components/common/RequiredMark';
import { ES_AR } from '@/shared/i18n/es_AR';
import { env } from '@/shared/lib/env';
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
  showSlugInput: boolean;
  setEditingSlug: (editing: boolean) => void;
  onSlugManualEdit: () => void;
  availability: SlugState;
  onSlugSuggestion: (slug: string) => void;
}

// The public address, as a line of text until someone asks to change it. It
// is derived from the name and almost nobody edits it, but it is also the
// ONE chance to choose it: `readOnly={isEdit}` means a slug decided here is
// the slug forever, because changing it later would break every link and
// printed QR already pointing at the old one. Hiding it outright would be
// deciding a permanent thing on the owner's behalf in silence; a field
// everyone must fill asks a question almost nobody has. So: shown, editable,
// one click away. On edit there is nothing to reveal — the input is
// read-only — so the line stays a line.
function SlugSection({
  register,
  errors,
  isEdit,
  slugValue,
  showSlugInput,
  setEditingSlug,
  onSlugManualEdit,
  availability,
  onSlugSuggestion,
}: SlugSectionProps) {
  return (
    <div className="-mt-1 space-y-2">
      {slugValue && !showSlugInput && (
        <PublicUrlLine
          slug={slugValue}
          onEdit={
            isEdit
              ? undefined
              : () => {
                  setEditingSlug(true);
                }
          }
        />
      )}
      <AvailabilityNote state={availability} slugValue={slugValue} onUse={onSlugSuggestion} />

      {/* Said before the field opens, not after the save. Changing this
          breaks every link already shared — WhatsApp, an Instagram bio, and
          the QR taped to the club's door — and nobody finds out until a
          client cannot book. The owner still decides; they decide knowing. */}
      {isEdit && showSlugInput && <p className="text-warning-text text-xs">{t.complex.slugChangeWarning}</p>}

      {showSlugInput && (
        <FormField
          label={
            <>
              {t.complex.slug} <RequiredMark />
            </>
          }
          htmlFor="slug"
          helpText={t.complex.slugHelp}
          error={errors.slug?.message}
        >
          <Input
            id="slug"
            className="max-w-80"
            aria-invalid={!!errors.slug}
            {...register('slug', {
              onChange: () => {
                if (!isEdit) onSlugManualEdit();
              },
            })}
          />
        </FormField>
      )}
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
  // Opened on request — or on its own when the server rejects the slug. A
  // "ya está en uso" pointing at a collapsed field names a problem and hides
  // the fix, so the error opens the thing it is about.
  const [editingSlug, setEditingSlug] = useState(false);
  const showSlugInput = editingSlug || !slugValue || !!errors.slug;
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
        showSlugInput={showSlugInput}
        setEditingSlug={setEditingSlug}
        onSlugManualEdit={onSlugManualEdit}
        availability={availability}
        onSlugSuggestion={onSlugSuggestion}
      />
    </div>
  );
}

/**
 * Where this complex's public page actually lives.
 *
 * The host used to be the literal string "app.vibe.com.ar/" sitting in the
 * translation file, which made it wrong everywhere the app is not that host —
 * a local dev server, a preview deploy, a custom domain — and made a hostname
 * something a translator could change. It comes from the same place the
 * canonical tag and the dashboard's share link take it from.
 *
 * The scheme is stripped because this is a label, not a link: "app.vibe.com.ar"
 * reads as an address, "https://app.vibe.com.ar" reads as a URL bar.
 */
function publicOrigin(): string {
  return env.VITE_APP_URL.replace(/^https?:\/\//, '').replace(/\/$/, '');
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

/**
 * The club's public address, as a line of text with a way in.
 *
 * `onEdit` is absent while editing an existing complex, where the field is
 * read-only and there is nothing to open.
 */
function PublicUrlLine({ slug, onEdit }: { slug: string; onEdit?: (() => void) | undefined }) {
  return (
    <p className="text-text-tertiary flex flex-wrap items-baseline gap-x-2 text-xs">
      <span className="truncate">
        {t.complex.slugPreviewLabel}{' '}
        <span className="text-primary-400 font-medium">
          {publicOrigin()}/{slug}
        </span>
      </span>
      {onEdit && (
        <button
          type="button"
          onClick={onEdit}
          className="focus-self hover:text-primary-400 underline transition-colors"
        >
          {t.common.edit}
        </button>
      )}
    </p>
  );
}
