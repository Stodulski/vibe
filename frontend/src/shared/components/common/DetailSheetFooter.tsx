import { Children, type ReactNode } from 'react';
import { SheetFooter } from '@/shared/components/ui/sheet';
import { cn } from '@/shared/lib/utils';

interface DetailSheetFooterProps {
  /** The one positive primary action (e.g. "Confirmar pago"): a single `Button`. */
  children?: ReactNode;
  /**
   * The destructive or cancelling action (e.g. "Cancelar evento"): a single
   * `Button variant="destructive-outline"`. Sits at the left from `sm`, and
   * under the primary one on mobile.
   */
  secondary?: ReactNode;
  /**
   * Keeps the actions full width and stacked on every size. Only for bottom
   * sheets, where the sheet already spans the screen; side drawers leave it off.
   */
  fullWidth?: boolean;
}

/**
 * The pinned footer of a detail drawer: the actions, never a header menu. The
 * destructive one (`secondary`) goes on the left and the positive primary one
 * (`children`) on the right. On mobile they stack at full width with the primary
 * on top. The footer shares the body's horizontal padding (`px-4 sm:px-6`).
 * Renders nothing when there is no action, so a drawer without one has no
 * empty footer bar. Use the standard `Button` variants and radius inside.
 */
export function DetailSheetFooter({ children, secondary, fullWidth = false }: DetailSheetFooterProps) {
  const hasPrimary = Children.toArray(children).length > 0;
  const hasSecondary = Children.toArray(secondary).length > 0;
  if (!hasPrimary && !hasSecondary) return null;

  const row = !fullWidth && hasSecondary;
  return (
    <SheetFooter
      className={cn(
        'px-4 sm:px-6 [&>button]:w-full',
        !fullWidth && !hasSecondary && 'sm:items-end sm:[&>button]:w-auto',
        row && 'flex-col-reverse sm:flex-row sm:items-center sm:justify-between sm:[&>button]:w-auto',
        fullWidth && hasSecondary && 'flex-col-reverse',
      )}
    >
      {secondary}
      {children}
    </SheetFooter>
  );
}
