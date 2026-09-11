import type { ReactNode } from 'react';
import { AlertCircle } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';
import { Label } from '@/shared/components/ui/label';
import { IconInput } from './IconInput';

interface FormFieldProps {
  /** Field label. Accepts JSX so callers can append an inline "(opcional)"/"*" marker. */
  label: ReactNode;
  /**
   * Wires the label to the input via `htmlFor`/`id` and derives the error
   * message's `id`. Required: a `label` with no `htmlFor` renders text next
   * to the input, not a label associated with it — screen readers announce
   * nothing when the input receives focus. `children` must render the actual
   * form control with a matching `id={htmlFor}`.
   */
  htmlFor: string;
  /**
   * Validation error message, typically `errors.x?.message` from React Hook
   * Form — whose type is `string | undefined`, not an omittable `string`, so
   * this is spelled out explicitly rather than left as a bare `error?:
   * string` (the two differ under `exactOptionalPropertyTypes`).
   */
  error?: string | undefined;
  /** Optional leading icon, rendered inside a focus-aware wrapper around `children`. */
  icon?: LucideIcon;
  /** Optional trailing content next to the label (e.g. a "max $X" hint), right-aligned. */
  labelSuffix?: ReactNode;
  /** Optional helper text rendered between the label and the input. */
  helpText?: ReactNode;
  /** Overrides the wrapper's vertical spacing (defaults to `space-y-1.5`). */
  className?: string;
  /** The actual input/select/textarea element(s) — this component only owns label+error chrome. */
  children: ReactNode;
}

/**
 * Shared label + input-slot + error chrome for form fields — extracted from the
 * hand-duplicated `space-y-1.5` + `Label` + `errors.x` pattern repeated across
 * feature forms. Every error message renders with `role="alert"` so it's
 * announced to screen readers (WCAG 2.1 AA), regardless of whether the
 * original site remembered to add it.
 *
 * The error sits ABOVE the input, not below it. Below is where it was, and
 * below is where a phone's on-screen keyboard covers it: the person is told
 * what is wrong in a line they cannot see while typing the correction. It
 * also reads in the wrong order — the field, then the complaint about the
 * field. Above, with an icon, it is read before the input it is about, and
 * the icon carries the meaning for anyone who does not see the red.
 */
export function FormField({
  label,
  htmlFor,
  error,
  icon: Icon,
  labelSuffix,
  helpText,
  className = 'space-y-1.5',
  children,
}: FormFieldProps) {
  const errorId = `${htmlFor}-error`;

  return (
    <div className={className}>
      {labelSuffix ? (
        <div className="flex items-baseline justify-between">
          <Label htmlFor={htmlFor} className="text-xs">
            {label}
          </Label>
          {labelSuffix}
        </div>
      ) : (
        <Label htmlFor={htmlFor} className="text-xs">
          {label}
        </Label>
      )}
      {helpText && <p className="text-micro text-text-tertiary">{helpText}</p>}
      {error && (
        <p id={errorId} role="alert" className="flex items-center gap-1.5 text-xs text-error-text">
          <AlertCircle className="size-3.5 shrink-0" aria-hidden="true" />
          {error}
        </p>
      )}
      {Icon ? (
        <IconInput icon={Icon} wrapperClassName="relative">
          {children}
        </IconInput>
      ) : (
        children
      )}
    </div>
  );
}
