import { Children, cloneElement, isValidElement, type ReactNode } from 'react';
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
  /**
   * The actual form control. A single element gets `id`, `aria-invalid` and
   * `aria-describedby` wired to this field automatically (see `wireControl`);
   * anything else — several elements, a fragment, bare text — is rendered
   * untouched and keeps whatever it declares itself.
   */
  children: ReactNode;
}

/** The three attributes this component wires onto its control. */
interface ControlA11yProps {
  id?: string | undefined;
  'aria-invalid'?: boolean | 'true' | 'false' | undefined;
  'aria-describedby'?: string | undefined;
  children?: ReactNode;
}

/**
 * Connects the control to its label and its error message.
 *
 * These three attributes are what turn a red border into something a screen
 * reader can report: `id` pairs the control with the `<label htmlFor>`,
 * `aria-invalid` says it is rejected, and `aria-describedby` points at the
 * message saying why. `FormField` computed the message's id and then left all
 * three to the caller — so 14 of its 33 consumers simply never wrote them, and
 * their errors were visible but not announced.
 *
 * Only a leaf is wired. Several fields hand this component a positioning
 * wrapper — `<div className="relative">` around an icon and the real input —
 * and giving the wrapper the `id` was actively harmful: the input inside
 * already carried it, so two elements claimed one id, and HTML resolves
 * `<label for>` to the FIRST element with that id and then labels nothing at
 * all because a `<div>` is not labelable. The field became invisible to
 * `getByLabel` and to a screen reader, which is what broke the settings E2E.
 * An element with children of its own is therefore left alone: the control is
 * somewhere inside it, and this component cannot tell which descendant it is.
 *
 * The child's own props still win. Every call site that already spells these
 * out keeps behaving exactly as it did, and a control that manages its own
 * `aria-describedby` (a field with help text as well as an error) is not
 * overwritten. A component that forwards nothing to the DOM — a Radix `Select`
 * root, whose attributes belong on its trigger — quietly ignores them, which
 * is why those fields still wire their trigger themselves.
 */
function wireControl(children: ReactNode, htmlFor: string, errorId: string, hasError: boolean): ReactNode {
  if (Children.count(children) !== 1 || !isValidElement<ControlA11yProps>(children)) return children;
  // A wrapper, not the control — see above.
  if (Children.count(children.props.children) > 0) return children;
  return cloneElement(children, {
    id: children.props.id ?? htmlFor,
    'aria-invalid': children.props['aria-invalid'] ?? (hasError ? true : undefined),
    'aria-describedby': children.props['aria-describedby'] ?? (hasError ? errorId : undefined),
  });
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
  const control = wireControl(children, htmlFor, errorId, !!error);

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
        <p id={errorId} role="alert" className="text-error-text flex items-center gap-1.5 text-xs">
          <AlertCircle className="size-3.5 shrink-0" aria-hidden="true" />
          {error}
        </p>
      )}
      {Icon ? (
        <IconInput icon={Icon} wrapperClassName="relative">
          {control}
        </IconInput>
      ) : (
        control
      )}
    </div>
  );
}
