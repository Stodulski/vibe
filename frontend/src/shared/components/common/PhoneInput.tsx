import { Input } from '@/shared/components/ui/input';
import { cn } from '@/shared/lib/utils';
import { DEFAULT_PHONE_PREFIX } from '@/shared/lib/constants';
import { usePhoneInputState } from './phone-input/usePhoneInputState';

interface PhoneInputProps {
  value: string;
  onChange: (value: string) => void;
  onBlur?: () => void;
  id?: string;
  placeholder?: string;
  inputClassName?: string;
  /**
   * Extra classes for the "+54" prefix box. The box has no height class of
   * its own — it stretches to match the input via `self-stretch` — so this
   * should never carry a height utility; doing so would override the stretch
   * and reintroduce the mismatch it fixes.
   */
  selectClassName?: string;
  disabled?: boolean;
  'aria-invalid'?: boolean;
  'aria-describedby'?: string | undefined;
  autoComplete?: string;
}

export function PhoneInput({
  value,
  onChange,
  onBlur,
  id,
  placeholder = '11 2345 6789',
  inputClassName,
  selectClassName,
  disabled,
  autoComplete = 'tel-national',
  ...ariaProps
}: PhoneInputProps) {
  const { localNumber, handleLocalChange } = usePhoneInputState({
    value,
    onChange,
  });

  return (
    // `items-stretch` (the flex default, made explicit here since it's load-
    // bearing) is what keeps the prefix box exactly as tall as the input:
    // neither one carries its own height class, so each stretches to the
    // row's height instead of two callers having to repeat matching height
    // classes that can drift apart — which is exactly how this broke before
    // (some callers overrode the input's height without also updating the
    // addon's).
    <div className="flex items-stretch">
      {/* Argentina is the only market this app serves — the prefix is fixed
          and never user-selectable, so this is a static label, not a
          control. `aria-hidden` keeps it out of the accessibility tree; the
          number input below still carries its own "Teléfono" label. */}
      <span
        aria-hidden="true"
        className={cn(
          'border-border-interactive bg-bg-subtle text-text-secondary flex shrink-0 items-center self-stretch rounded-l-xl border border-r-0 px-2.5 text-sm select-none',
          disabled && 'opacity-50',
          selectClassName,
        )}
      >
        {DEFAULT_PHONE_PREFIX}
      </span>
      <Input
        id={id}
        type="tel"
        inputMode="tel"
        value={localNumber}
        onChange={handleLocalChange}
        onBlur={onBlur}
        placeholder={placeholder}
        autoComplete={autoComplete}
        disabled={disabled}
        className={cn('rounded-l-none', inputClassName)}
        aria-invalid={ariaProps['aria-invalid']}
        aria-describedby={ariaProps['aria-describedby']}
      />
    </div>
  );
}
