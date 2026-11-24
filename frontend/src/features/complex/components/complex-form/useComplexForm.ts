import { useEffect, useRef } from 'react';
import { useWatch, type UseFormReturn } from 'react-hook-form';
import { useAppForm } from '@/shared/lib/form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createComplexSchema, updateComplexSchema, type CreateComplexDto } from '../../schemas/complex.schema';
import { useCreateComplex } from '../../hooks/useCreateComplex';
import { useUpdateComplex } from '../../hooks/useUpdateComplex';
import type { Complex } from '@/shared/types/api.types';
import { blankToUndefined, slugify } from './utils';
import { applyServerFieldErrors } from '@/shared/lib/serverFieldErrors';
import { isVersionConflict } from '@/shared/lib/ApiError';

function buildDefaultValues(complex: Complex | undefined) {
  if (complex) {
    return {
      name: complex.name,
      slug: complex.slug,
      formatted_address: `${complex.address}, ${complex.city}, ${complex.province}`,
      address: complex.address,
      city: complex.city,
      province: complex.province,
      latitude: complex.latitude ?? undefined,
      longitude: complex.longitude ?? undefined,
      phone: complex.phone,
      email: complex.email ?? '',
      // Stored as the bare handle; shown the way people write it.
      instagram: complex.instagram ? `@${complex.instagram}` : '',
      deposit_percentage: complex.deposit_percentage,
      cancellation_hours: complex.cancellation_hours,
      // Runtime-checked: a complex cached from before this field existed has
      // no `amenities` key, and the checkbox group would read undefined.
      amenities: Array.isArray(complex.amenities) ? complex.amenities : [],
      description: complex.description ?? '',
    };
  }
  return {
    formatted_address: '',
    address: '',
    city: '',
    province: '',
    latitude: undefined,
    longitude: undefined,
    phone: '',
    deposit_percentage: 30,
    cancellation_hours: 24,
    amenities: [],
  };
}

interface UseComplexFormArgs {
  complex?: Complex | undefined;
  onSuccess?: ((created?: Complex) => void) | undefined;
  /**
   * Called with a field name the server rejected, so a caller that hides that
   * field behind a disclosure can open it before the error is pointed at.
   */
  revealField?: (field: string) => void;
}

// `email`/`latitude`/`longitude` on `CreateComplexRequest`/
// `UpdateComplexRequest` are absent-or-present, not present-with-`undefined`
// — only include each when it has a value.
//
// `instagram` is sent as typed: the server reduces a link or an @handle to the
// bare handle. Blank is left out of a create, but sent on an edit, where
// leaving it out would keep the account the owner just erased.
function cleanComplexPayload(data: CreateComplexDto, isEdit: boolean) {
  // Destructure slug/formatted_address out — slug is sent explicitly by both
  // callers below (the field is editable in both create and edit mode);
  // formatted_address is a UI-only field never sent to the API.
  // `description` is only editable once the complex exists: the create
  // endpoint does not take it, so it is sent on an edit and dropped otherwise.
  const { slug, formatted_address, email, instagram, latitude, longitude, description, ...rest } = data;
  const cleanEmail = blankToUndefined(email);
  const cleanInstagram = isEdit ? instagram : blankToUndefined(instagram);
  const cleaned = {
    ...rest,
    ...(cleanEmail !== undefined ? { email: cleanEmail } : {}),
    ...(cleanInstagram !== undefined ? { instagram: cleanInstagram } : {}),
    ...(latitude !== undefined ? { latitude } : {}),
    ...(longitude !== undefined ? { longitude } : {}),
    ...(isEdit && description !== undefined ? { description } : {}),
  };
  return { slug, cleaned };
}

/**
 * Tracks which copy of the complex the form's fields were filled from.
 *
 * `baseVersion` is sent back with every save so the server refuses it if the
 * row moved since. It is NOT simply `complex.version`: a focus refetch can
 * bring a newer copy while the owner is typing, and adopting its version would
 * let this save silently overwrite that other edit.
 *
 * A newer copy refills the form when nothing typed would be lost, or right
 * after a stale-version 409 (`reloadOnNextCopy`), as the conflict toast tells
 * the owner, so the next save carries a version the server accepts.
 */
function useCopyVersion(complex: Complex | undefined, form: UseFormReturn<CreateComplexDto>) {
  const { isDirty } = form.formState;
  const baseVersion = useRef(complex?.version);
  const reloadOnNextCopy = useRef(false);

  useEffect(() => {
    if (!complex || complex.version === baseVersion.current) return;
    if (isDirty && !reloadOnNextCopy.current) return;
    reloadOnNextCopy.current = false;
    baseVersion.current = complex.version;
    form.reset(buildDefaultValues(complex));
  }, [complex, isDirty, form]);

  return {
    /**
     * The `version` key to send, or none: a copy without a real row version
     * (rows start at 1) sends no precondition rather than a 0 the API refuses.
     */
    precondition: (): { version?: number } => {
      const version = baseVersion.current;
      return version !== undefined && version >= 1 ? { version } : {};
    },
    /** The save went through: the server's copy is now the one on screen. */
    saved: (version: number) => {
      baseVersion.current = version;
    },
    onError: (error: unknown) => {
      if (isVersionConflict(error)) reloadOnNextCopy.current = true;
    },
  };
}

export function useComplexForm({ complex, onSuccess, revealField }: UseComplexFormArgs) {
  const isEdit = !!complex;
  const slugManuallyEdited = useRef(false);
  const createMutation = useCreateComplex();
  const updateMutation = useUpdateComplex(complex?.id ?? '');
  const mutation = isEdit ? updateMutation : createMutation;

  const form = useAppForm<CreateComplexDto>({
    resolver: zodResolver(isEdit ? updateComplexSchema : createComplexSchema),
    defaultValues: buildDefaultValues(complex),
  });
  const { setValue, control } = form;
  const copyVersion = useCopyVersion(complex, form);

  // `useWatch` (a subscription) rather than `watch('slug')` (a render-time
  // call): `watch()` returns a value React Compiler cannot safely memoize —
  // see `react-hooks/incompatible-library` — because it reads live from the
  // form's internal store outside React's own state, not through a
  // subscription React can track.
  const slugValue = useWatch({ control, name: 'slug' });

  /**
   * Runs from the name field's own `onChange`, not from an effect watching
   * `watch('name')`. That effect re-rendered the whole form on every
   * keystroke and derived the slug a render later, so name and slug were
   * visibly out of sync for one frame; deriving it in the same handler that
   * changes the name keeps them in the same update.
   */
  const onNameChange = (name: string) => {
    if (!isEdit && !slugManuallyEdited.current) {
      setValue('slug', slugify(name));
    }
  };

  // Server-side field errors land ON their field, not only in a toast.
  //
  // A taken public URL is the one the server alone can detect — two owners
  // can pick "club-norte" a second apart and nothing on this client knows —
  // and it used to surface as a toast beside a slug input that is now
  // collapsed behind "Editar". A message about a field nobody can see is
  // worse than no message: it names a problem and hides the fix. `setError`
  // puts it under the input AND `revealField` opens the group.
  const onMutationError = (error: unknown) => {
    copyVersion.onError(error);
    applyServerFieldErrors(form, error, { onField: revealField });
  };

  const onSubmit = (data: CreateComplexDto) => {
    const { slug, cleaned } = cleanComplexPayload(data, isEdit);

    // Branched instead of calling through a `mutation = isEdit ? update : create`
    // union: create and update hit different endpoints with differently-typed
    // request bodies (create requires most fields, update makes them all
    // optional), and a shared `mutation.mutate(...)` call needed a
    // `@ts-expect-error` to paper over that mismatch — one that would have
    // stayed silent through a real payload type change in either mutation.
    if (isEdit) {
      updateMutation.mutate(
        { ...copyVersion.precondition(), slug, ...cleaned },
        {
          onSuccess: (result) => {
            copyVersion.saved(result.complex.version);
            onSuccess?.(result.complex);
          },
          onError: onMutationError,
        },
      );
    } else {
      createMutation.mutate(
        { slug, ...cleaned },
        {
          onSuccess: (result) => {
            copyVersion.saved(result.complex.version);
            onSuccess?.(result.complex);
          },
          onError: onMutationError,
        },
      );
    }
  };

  return {
    form,
    isEdit,
    slugValue,
    mutation,
    onSubmit,
    onNameChange,
    onSlugManualEdit: () => {
      slugManuallyEdited.current = true;
    },
  };
}
