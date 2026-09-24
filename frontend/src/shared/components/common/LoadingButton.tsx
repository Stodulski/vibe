import type { ComponentProps } from 'react';
import { Loader2 } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { cn } from '@/shared/lib/utils';

interface LoadingButtonProps extends ComponentProps<typeof Button> {
  /**
   * Shows a centered spinner over the button while it works. The button
   * keeps the width its label gives it: the label stays in the layout
   * (hidden with `opacity-0`, never removed or `visibility:hidden`, which
   * would drop it from the accessibility tree and leave the button
   * unnamed) while the spinner is absolutely positioned on top of it.
   */
  loading?: boolean;
  /**
   * Optional text announced to screen readers (visually hidden, `aria-live`)
   * while `loading` is true — e.g. "Iniciando sesión...". `aria-busy`
   * already communicates the state; pass this only when the action itself
   * is worth narrating.
   */
  loadingText?: string;
}

/**
 * Wraps the shadcn `Button` (`ui/button.tsx`) instead of editing it: a
 * loading state that must overlay a spinner and toggle `disabled`/`aria-busy`
 * is component behavior, not a CVA style variant, so it belongs in a
 * composed wrapper (same pattern as `AppDialog` over `ui/dialog.tsx`).
 *
 * Every call site that used to swap its `<Button>` children for a spinner —
 * which shrinks the button to the spinner's width — should render its normal
 * children here and pass `loading` instead.
 */
export function LoadingButton({
  loading = false,
  loadingText,
  disabled,
  className,
  children,
  ...props
}: LoadingButtonProps) {
  return (
    <Button aria-busy={loading} disabled={!!disabled || loading} className={cn('relative', className)} {...props}>
      <span className={cn('inline-flex items-center justify-center gap-2', loading && 'opacity-0')}>{children}</span>
      {loading && (
        <span className="absolute inset-0 flex items-center justify-center" aria-hidden="true">
          <Loader2 className="size-4 animate-spin" />
        </span>
      )}
      {loading && loadingText && (
        <span className="sr-only" aria-live="polite">
          {loadingText}
        </span>
      )}
    </Button>
  );
}
