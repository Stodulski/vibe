import { Banknote } from 'lucide-react';
import { Input } from '@/shared/components/ui/input';
import { useMoneyInput } from '@/shared/hooks/useMoneyInput';
import { cn } from '@/shared/lib/utils';

interface MoneyInputProps {
  /** Pesos; `undefined` renders a blank field. */
  value: number | undefined;
  onChange: (pesos: number | undefined) => void;
  id?: string | undefined;
  placeholder?: string | undefined;
  disabled?: boolean | undefined;
  'aria-label'?: string | undefined;
  'aria-invalid'?: boolean | undefined;
  'aria-describedby'?: string | undefined;
  /** Width/layout only. The look (height, icon, padding) is fixed so every money field matches. */
  className?: string | undefined;
}

/**
 * The one money input of the app: Banknote icon + the shared `Input`, with
 * es-AR thousands formatting and caret handling from `useMoneyInput`. Every
 * price/amount field renders this, so they all look and behave the same;
 * labels, hints and errors stay with the caller (see `MoneyPesosField`).
 *
 * `type="text"`, not `type="number"`: a number input cannot render "150.000".
 * Callers whose form uses native validation should set `noValidate`; the
 * caller's own schema reports problems.
 */
export function MoneyInput({ value, onChange, className, ...rest }: MoneyInputProps) {
  const { displayValue, inputRef, handleChange, handleBlur } = useMoneyInput({ value, onChange });

  return (
    <div className={cn('relative', className)}>
      <Banknote className="text-text-tertiary absolute top-1/2 left-3 size-4 -translate-y-1/2" aria-hidden="true" />
      <Input
        type="text"
        inputMode="numeric"
        autoComplete="off"
        className="pl-9"
        ref={inputRef}
        value={displayValue}
        onChange={handleChange}
        onBlur={handleBlur}
        {...rest}
      />
    </div>
  );
}
