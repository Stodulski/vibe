import { Input } from '@/shared/components/ui/input';
import { AddressPredictionList } from './address-input/AddressPredictionList';
import { useAddressAutocomplete } from './address-input/useAddressAutocomplete';
import type { AddressSelection } from './address-input/types';

export type { AddressSelection };

interface AddressInputProps {
  value: string;
  confirmed: boolean;
  onChange: (value: string) => void;
  onSelect: (details: AddressSelection) => void;
  onClear: () => void;
  placeholder?: string;
  'aria-invalid'?: boolean;
  id?: string;
}

interface AddressComboboxInputProps {
  id: string | undefined;
  value: string;
  placeholder: string | undefined;
  open: boolean;
  activeIndex: number;
  predictions: unknown[];
  onInputChange: (value: string) => void;
  onKeyDown: React.KeyboardEventHandler<HTMLInputElement>;
  setOpen: (open: boolean) => void;
  rest: Record<string, unknown>;
}

function AddressComboboxInput({
  id,
  value,
  placeholder,
  open,
  activeIndex,
  predictions,
  onInputChange,
  onKeyDown,
  setOpen,
  rest,
}: AddressComboboxInputProps) {
  return (
    <Input
      id={id}
      value={value}
      placeholder={placeholder}
      autoComplete="off"
      onChange={(e) => {
        onInputChange(e.target.value);
      }}
      onFocus={() => {
        if (predictions.length > 0) setOpen(true);
      }}
      onKeyDown={onKeyDown}
      role="combobox"
      aria-expanded={open}
      aria-autocomplete="list"
      aria-controls={open ? 'address-listbox' : undefined}
      aria-activedescendant={activeIndex >= 0 ? `address-option-${String(activeIndex)}` : undefined}
      {...rest}
    />
  );
}

export function AddressInput({
  value,
  confirmed,
  onChange,
  onSelect,
  onClear,
  placeholder,
  id,
  ...props
}: AddressInputProps) {
  const {
    predictions,
    open,
    activeIndex,
    loading,
    error,
    containerRef,
    handleSelect,
    handleKeyDown,
    handleInputChange,
    setOpen,
    setActiveIndex,
  } = useAddressAutocomplete({ confirmed, onChange, onSelect, onClear });

  return (
    <div ref={containerRef} className="relative">
      <AddressComboboxInput
        id={id}
        value={value}
        placeholder={placeholder}
        open={open}
        activeIndex={activeIndex}
        predictions={predictions}
        onInputChange={handleInputChange}
        onKeyDown={handleKeyDown}
        setOpen={setOpen}
        rest={props}
      />

      {loading && (
        <div className="absolute top-1/2 right-3 -translate-y-1/2">
          <div className="border-primary-500/30 border-t-primary-500 size-4 animate-spin rounded-full border-2" />
        </div>
      )}

      {open && (
        <AddressPredictionList
          predictions={predictions}
          activeIndex={activeIndex}
          onSelect={(p) => {
            void handleSelect(p);
          }}
          onHover={setActiveIndex}
        />
      )}

      {error && (
        <p role="alert" className="text-error-text mt-1 text-xs">
          {error}
        </p>
      )}
    </div>
  );
}
