import { useState } from 'react';
import { ComplexForm, ImageUpload, useDeleteComplex } from '@/features/complex';
import { DangerZone } from '@/shared/components/common/DangerZone';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Complex } from '@/shared/types/api.types';

const t = ES_AR;

interface SettingsGeneralTabProps {
  complex: Complex;
}

/**
 * Everything the public sees about a complex, in one place.
 *
 * The logo and cover used to live in a tab of their own holding two controls.
 * They belong here: an owner filling in the name and the public URL is doing
 * the same job as the one uploading the logo, and splitting it across two
 * screens made both look emptier than the work deserved.
 *
 * Deleting the complex stays last, in its own danger zone, behind a confirmation.
 */
export function SettingsGeneralTab({ complex }: SettingsGeneralTabProps) {
  const [deleteOpen, setDeleteOpen] = useState(false);
  const deleteComplex = useDeleteComplex();

  return (
    <div className="space-y-8">
      {/* Images first: they are the top of the public page, so they are the
          top of the form that makes it. Same measure as the fields below, and
          centred in the card like them. */}
      <div className="mx-auto max-w-xl">
        <ImageUpload complex={complex} />
      </div>

      {/* No wrapper heading: the form names its own groups now, and the outer
          one repeated "Ubicación y contacto" word for word above the inner. */}
      <ComplexForm complex={complex} />

      <DangerZone
        description={t.complex.deleteComplexDescription}
        actionLabel={t.complex.deleteComplex}
        onAction={() => {
          setDeleteOpen(true);
        }}
      />

      <ConfirmDialog
        open={deleteOpen}
        onClose={() => {
          setDeleteOpen(false);
        }}
        onConfirm={() => {
          deleteComplex.mutate(complex.id);
        }}
        title={`${t.complex.deleteComplex} "${complex.name}"`}
        description={t.complex.deleteComplexConfirmDescription}
        confirmLabel={t.complex.deleteComplexConfirmLabel}
        variant="destructive"
        isLoading={deleteComplex.isPending}
      />
    </div>
  );
}
