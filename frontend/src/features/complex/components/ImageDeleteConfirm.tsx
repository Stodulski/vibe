import { Trash2 } from 'lucide-react';
import { useState } from 'react';
import { Button } from '@/shared/components/ui/button';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Delete-confirmation trigger + dialog shared by `LogoCard` and
 * `CoverCard` (slice 10, max-lines decomposition — both cards had an
 * identical block, so this also removes a DRY violation).
 */
export function ImageDeleteConfirm({
  label,
  disabled,
  onConfirm,
}: {
  label: string;
  disabled: boolean;
  onConfirm: () => void;
}) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        disabled={disabled}
        aria-label={label}
        onClick={() => {
          setOpen(true);
        }}
      >
        <Trash2 className="text-error-text size-3.5" />
      </Button>
      <ConfirmDialog
        open={open}
        onClose={() => {
          setOpen(false);
        }}
        onConfirm={() => {
          setOpen(false);
          onConfirm();
        }}
        title={t.complex.deleteImageConfirm}
        description={t.complex.deleteImageConfirmDescription}
        confirmLabel={t.complex.deleteImage}
        variant="destructive"
      />
    </>
  );
}
