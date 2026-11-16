import { Loader2 } from 'lucide-react';
import { AlertDialogAction, AlertDialogCancel, AlertDialogFooter } from '../../ui/alert-dialog';

interface ConfirmDialogFooterProps {
  isDestructive: boolean;
  isLoading: boolean;
  cancelLabel: string;
  confirmLabel: string;
  onConfirm: () => void;
}

export function ConfirmDialogFooter({
  isDestructive,
  isLoading,
  cancelLabel,
  confirmLabel,
  onConfirm,
}: ConfirmDialogFooterProps) {
  return (
    <AlertDialogFooter className="mt-6 gap-3 sm:flex-row">
      <AlertDialogCancel disabled={isLoading}>{cancelLabel}</AlertDialogCancel>
      <AlertDialogAction
        variant={isDestructive ? 'destructive' : 'default'}
        onClick={(e) => {
          e.preventDefault();
          onConfirm();
        }}
        disabled={isLoading}
      >
        {isLoading && <Loader2 className="size-4 animate-spin" aria-hidden="true" />}
        {confirmLabel}
      </AlertDialogAction>
    </AlertDialogFooter>
  );
}
