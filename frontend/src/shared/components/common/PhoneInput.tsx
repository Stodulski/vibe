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
    <div className="flex">
      {/* Argentina is the only market this app serves — the prefix is fixed
          and never user-selectable, so this is a static label, not a
          control. `aria-hidden` keeps it out of the accessibility tree; the
          number input below still carries its own "Teléfono" label. */}
      <span
        aria-hidden="true"
        className={cn(
          'flex h-10 shrink-0 select-none items-center rounded-l-xl border border-r-0 border-border-interactive bg-bg-subtle px-2.5 text-sm text-text-secondary',
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
