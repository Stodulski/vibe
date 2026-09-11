import { Loader2 } from 'lucide-react';
import { AlertDialogAction, AlertDialogCancel, AlertDialogFooter } from '@/shared/components/ui/alert-dialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';

interface ConfirmDialogFooterProps {
  isDestructive: boolean;
  isLoading: boolean;
  confirmLabel: string;
  onConfirm: () => void;
}

export function ConfirmDialogFooter({ isDestructive, isLoading, confirmLabel, onConfirm }: ConfirmDialogFooterProps) {
  return (
    <AlertDialogFooter className="mt-6 gap-3 sm:flex-row">
      <AlertDialogCancel
        disabled={isLoading}
        className={cn(
          'rounded-xl border-border-subtle bg-transparent text-nav font-medium text-text-secondary',
          'hover:bg-bg-elevated hover:text-text-primary',
          'transition-colors duration-150',
        )}
      >
        {ES_AR.common.cancel}
      </AlertDialogCancel>
      <AlertDialogAction
        onClick={(e) => {
          e.preventDefault();
          onConfirm();
        }}
        disabled={isLoading}
        className={cn(
          'rounded-xl text-nav font-semibold transition-colors duration-150',
          isDestructive
            ? 'bg-error-text text-white hover:bg-error-text/90'
            : // Glow color reuses the existing --color-primary-500 token (CSS
              // relative color syntax) instead of hardcoding rgb(29,185,84) again.
              'bg-primary-500 text-bg-base hover:bg-primary-400 shadow-[0_0_12px_rgb(from_var(--color-primary-500)_r_g_b/15%)]',
        )}
      >
        {isLoading && <Loader2 className="size-4 animate-spin" aria-hidden="true" />}
        {confirmLabel}
      </AlertDialogAction>
    </AlertDialogFooter>
  );
}
