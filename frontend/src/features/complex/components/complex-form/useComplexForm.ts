import { useRef } from 'react';
import { useWatch } from 'react-hook-form';
import { useAppForm } from '@/shared/lib/form';
import { zodResolver } from '@hookform/resolvers/zod';
import { createComplexSchema, updateComplexSchema, type CreateComplexDto } from '../../schemas/complex.schema';
import { useCreateComplex } from '../../hooks/useCreateComplex';
import { useUpdateComplex } from '../../hooks/useUpdateComplex';
import type { Complex } from '@/shared/types/api.types';
import { blankToUndefined, slugify } from './utils';
import { applyServerFieldErrors } from '@/shared/lib/serverFieldErrors';

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
      deposit_percentage: complex.deposit_percentage,
      cancellation_hours: complex.cancellation_hours,
      // Runtime-checked: a complex cached from before this field existed has
      // no `amenities` key, and the checkbox group would read undefined.
      amenities: Array.isArray(complex.amenities) ? complex.amenities : [],
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
function cleanComplexPayload(data: CreateComplexDto) {
  // Destructure slug/formatted_address out — slug is auto-generated on create
  // and excluded on edit; formatted_address is a UI-only field never sent to the API.
  const { slug, formatted_address, email, latitude, longitude, ...rest } = data;
  const cleanEmail = blankToUndefined(email);
  const cleaned = {
    ...rest,
    ...(cleanEmail !== undefined ? { email: cleanEmail } : {}),
    ...(latitude !== undefined ? { latitude } : {}),
    ...(longitude !== undefined ? { longitude } : {}),
  };
  return { slug, cleaned };
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
    applyServerFieldErrors(form, error, { onField: revealField });
  };

  const onSubmit = (data: CreateComplexDto) => {
    const { slug, cleaned } = cleanComplexPayload(data);

    // Branched instead of calling through a `mutation = isEdit ? update : create`
    // union: create and update take differently-shaped payloads (only create
    // sends `slug`), and a shared `mutation.mutate(...)` call needed a
    // `@ts-expect-error` to paper over that mismatch — one that would have
    // stayed silent through a real payload type change in either mutation.
    if (isEdit) {
      updateMutation.mutate(cleaned, {
        onSuccess: (result) => onSuccess?.(result.complex),
        onError: onMutationError,
      });
    } else {
      createMutation.mutate(
        { slug, ...cleaned },
        {
          onSuccess: (result) => onSuccess?.(result.complex),
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
