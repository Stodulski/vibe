import { Children, type ReactNode } from 'react';
import { SheetFooter } from '@/shared/components/ui/sheet';
import { cn } from '@/shared/lib/utils';

interface DetailSheetFooterProps {
  /** The one positive primary action (e.g. "Confirmar pago"): a single `Button`. */
  children?: ReactNode;
  /**
   * Keeps the action full width on every size. Only for bottom sheets, where
   * the sheet already spans the screen; side drawers leave it off.
   */
  fullWidth?: boolean;
}

/**
 * The pinned footer of a detail drawer, for its positive primary action only:
 * destructive and secondary actions live in the header's ⋯ menu. The footer
 * shares the body's horizontal padding (`px-4 sm:px-6`); the button spans the
 * width on mobile and sits at the right, as wide as its text, from `sm`.
 * Renders nothing when there is no action, so a drawer without one has no
 * empty footer bar. Use the standard `Button` variants and radius inside.
 */
export function DetailSheetFooter({ children, fullWidth = false }: DetailSheetFooterProps) {
  if (Children.toArray(children).length === 0) return null;

  return (
    <SheetFooter className={cn('px-4 sm:px-6 [&>button]:w-full', !fullWidth && 'sm:items-end sm:[&>button]:w-auto')}>
      {children}
    </SheetFooter>
  );
}
