import { Trash2 } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { AlertDialog, AlertDialogContent } from '@/shared/components/common/AppAlertDialog';
import {
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/shared/components/ui/alert-dialog';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * Delete-confirmation trigger + dialog shared by `LogoCard` and
 * `CoverCard` (slice 10, max-lines decomposition — both cards had an
 * identical block, so this also removes a DRY violation).
 */
export function ImageDeleteConfirm({ disabled, onConfirm }: { disabled: boolean; onConfirm: () => void }) {
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>
        <Button variant="ghost" size="sm" disabled={disabled}>
          <Trash2 className="size-3.5 text-error-text" />
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t.complex.deleteImageConfirm}</AlertDialogTitle>
          <AlertDialogDescription>{t.complex.deleteImageConfirmDescription}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>{t.common.cancel}</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm}>{t.complex.deleteImage}</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
