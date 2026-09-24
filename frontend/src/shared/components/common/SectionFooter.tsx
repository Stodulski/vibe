import type { ComponentProps, ReactNode } from 'react';
import { Button } from '@/shared/components/ui/button';
import { LoadingButton } from '@/shared/components/common/LoadingButton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';

const t = ES_AR;

type ButtonVariant = ComponentProps<typeof Button>['variant'];
type ButtonSize = ComponentProps<typeof Button>['size'];

interface SectionFooterProps {
  /** Omit to render a submit-only footer (no cancel button). */
  onCancel?: () => void;
  cancelLabel?: string;
  cancelDisabled?: boolean;
  /** Content of the submit button when not pending. */
  submitLabel?: ReactNode;
  /** When true, replaces submitLabel with a spinner and disables the submit button. */
  pending?: boolean;
  submitDisabled?: boolean;
  submitVariant?: ButtonVariant;
  /**
   * Where the buttons sit on desktop. Defaults to 'end' — the shape every
   * modal in this app already has, and changing that here would move ten
   * dialogs' worth of buttons at once.
   *
   * Pass 'start' on a page form: reading runs down the left edge, so a
   * right-aligned submit is missed on a wide screen and by anyone using a
   * screen magnifier.
   */
  align?: 'start' | 'end';
  /**
   * Rendered next to the submit button. For an action that belongs to the same
   * form but is not part of finishing it — a destructive one behind a menu,
   * say, which has no business being a third button in the row.
   */
  extra?: ReactNode;
  submitSize?: ButtonSize;
  /** Only needed outside a `<form>` using native submit-on-Enter. */
  onSubmit?: () => void;
  className?: string;
  /**
   * Fully custom footer content, replacing the cancel/submit shape built from
   * the flag props above. Prefer composing with `SectionFooterCancel` and
   * `SectionFooterSubmit` for new call sites — the flag props stay only for
   * existing callers built before those existed.
   */
  children?: ReactNode;
}

/**
 * Shared cancel/submit action bar: full-width, stacked (submit on top) on
 * mobile; right-aligned in a row on desktop. See ModalFooter/FormFooter for
 * the pattern this replaces.
 */
export function SectionFooter({
  onCancel,
  cancelLabel,
  cancelDisabled,
  submitLabel,
  pending,
  submitDisabled,
  submitVariant,
  submitSize,
  align = 'end',
  extra,
  onSubmit,
  className,
  children,
}: SectionFooterProps) {
  return (
    <div
      className={cn(
        'border-border-subtle flex flex-col-reverse gap-3 border-t pt-4 sm:flex-row',
        // With a trailing action there is no stacking: the submit takes the
        // room that is left and the menu sits beside it. Below it, alone under
        // a full-width button, the menu reads as something left over.
        extra && 'flex-row-reverse items-center',
        align === 'end' ? 'sm:justify-end' : 'sm:flex-row-reverse sm:justify-end',
        className,
      )}
    >
      {children ?? (
        <>
          {/* First in the DOM so it lands LAST visually: the left-aligned
              variant is row-reverse, which paints the first child rightmost.
              The submit has to lead, and the menu trails it. */}
          {extra}
          {onCancel && (
            <Button
              type="button"
              variant="outline"
              onClick={onCancel}
              disabled={cancelDisabled}
              className="w-full sm:w-auto"
            >
              {cancelLabel ?? t.common.cancel}
            </Button>
          )}
          <LoadingButton
            type={onSubmit ? 'button' : 'submit'}
            variant={submitVariant}
            size={submitSize}
            onClick={onSubmit}
            // Either reason disables the button, so this is a real OR — not
            // the nullish coalescing the linter suggests, which would return
            // `pending` whenever it is `false` and never look at
            // `submitDisabled` at all. Both are `boolean | undefined`;
            // coercing them first says the same thing without the ambiguity.
            disabled={!!submitDisabled}
            loading={!!pending}
            // `w-full` would eat the whole row and push a trailing action off
            // the end of it — which is exactly what happened to the overflow
            // menu. With one present the submit takes what is left instead.
            className={cn(extra ? 'flex-1 sm:w-auto sm:flex-none' : 'w-full sm:w-auto')}
          >
            {submitLabel}
          </LoadingButton>
        </>
      )}
    </div>
  );
}

type SectionFooterButtonProps = ComponentProps<typeof Button>;

/**
 * Composable cancel button for a `SectionFooter` used via `children`, e.g.
 * `<SectionFooter><SectionFooterCancel onClick={onClose}>Cancelar</SectionFooterCancel>
 * <SectionFooterSubmit pending={pending}>Guardar</SectionFooterSubmit></SectionFooter>`.
 */
export function SectionFooterCancel({
  className,
  variant = 'outline',
  type = 'button',
  ...props
}: SectionFooterButtonProps) {
  return <Button type={type} variant={variant} className={cn('w-full sm:w-auto', className)} {...props} />;
}

interface SectionFooterSubmitProps extends SectionFooterButtonProps {
  /** Replaces the button content with a spinner and disables it. */
  pending?: boolean;
}

/** Composable submit button for a `SectionFooter` used via `children` — see `SectionFooterCancel`. */
export function SectionFooterSubmit({
  className,
  type = 'submit',
  pending,
  disabled,
  children,
  ...props
}: SectionFooterSubmitProps) {
  return (
    <LoadingButton
      type={type}
      disabled={!!disabled}
      loading={!!pending}
      className={cn('w-full sm:w-auto', className)}
      {...props}
    >
      {children}
    </LoadingButton>
  );
}
