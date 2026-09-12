import { AlertDialog, AlertDialogContent } from '@/shared/components/common/AppAlertDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import { ConfirmDialogHeader } from './confirm-dialog/ConfirmDialogHeader';
import { ConfirmDialogFooter } from './confirm-dialog/ConfirmDialogFooter';

interface ConfirmDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  description: string;
  confirmLabel?: string;
  variant?: 'destructive' | 'default';
  isLoading?: boolean;
}

export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmLabel = ES_AR.common.confirm,
  variant = 'default',
  isLoading = false,
}: ConfirmDialogProps) {
  const isDestructive = variant === 'destructive';

  return (
    <AlertDialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      {/* No `animate-fade-in-scale` here. That class sets the `animation`
          shorthand from an unlayered rule, and unlayered CSS outranks anything
          in `@layer utilities` — so it pinned `animation-name` and Radix's own
          `data-[state=closed]:animate-out` never got to swap it to `exit`.
          Radix decides there is an exit animation by watching that name change,
          so it concluded there was none and tore the dialog out on the spot.
          `AlertDialogContent` already ships the same 200ms fade-and-scale, with
          the closing half included. */}
      <AlertDialogContent
        // The variant, said in the DOM rather than only in a colour — the same
        // way `Button` carries its own. Whether a confirmation is destructive
        // is a fact about the decision being asked for, and a caller that gets
        // it wrong is a real defect; without this, the only way to observe it
        // was to read a Tailwind token off a class list, which breaks on any
        // restyle and says nothing about behaviour.
        data-variant={variant}
        className={cn(
          'bg-bg-subtle overflow-hidden rounded-2xl border p-0',
          // Reuses the existing --shadow-lg token instead of a near-duplicate literal.
          'border-border-subtle shadow-lg',
          'sm:max-w-md',
        )}
      >
        {/* Accent top border */}
        <div className={cn('h-[2px] w-full', isDestructive ? 'bg-error-text' : 'bg-primary-500')} />

        <div className="px-6 pt-5 pb-6">
          <ConfirmDialogHeader isDestructive={isDestructive} title={title} description={description} />
          <ConfirmDialogFooter
            isDestructive={isDestructive}
            isLoading={isLoading}
            confirmLabel={confirmLabel}
            onConfirm={onConfirm}
          />
        </div>
      </AlertDialogContent>
    </AlertDialog>
  );
}
