import { useId } from 'react';
import { FormField, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@vibe/ui';
import { type AgendaFilter, type ModalityOption } from './agendaModel';

/** The words of the filter: its label and its "all" option. */
export interface AgendaFilterCopy {
  modality: string;
  allModalities: string;
}

interface AgendaFiltersProps {
  copy: AgendaFilterCopy;
  /** The modalities the rows have; "all" is always offered first. */
  modalities: readonly ModalityOption[];
  filter: AgendaFilter;
  onChange: (filter: AgendaFilter) => void;
}

/** The modality the chosen value stands for; anything that is not a listed modality is "all". */
function modalityOf(value: string, modalities: readonly ModalityOption[]): AgendaFilter['modality'] {
  return modalities.find((option) => option.value === value)?.value ?? 'all';
}

interface FilterOption {
  value: string;
  label: string;
}

interface FilterSelectProps {
  id: string;
  label: string;
  value: string;
  options: readonly FilterOption[];
  onValueChange: (value: string) => void;
}

/** One filter: its label above a Select of its options, at a compact width from `sm` up. */
function FilterSelect({ id, label, value, options, onValueChange }: FilterSelectProps) {
  return (
    <div className="min-w-0 sm:w-60">
      <FormField label={label} htmlFor={id}>
        <Select value={value} onValueChange={onValueChange}>
          <SelectTrigger id={id}>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {options.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormField>
    </div>
  );
}

/**
 * The modality of an agenda, as a select with its label above it: the full width
 * on a phone, a compact width on a desktop. The list is short and in date order,
 * so there is no day filter.
 */
export function AgendaFilters({ copy, modalities, filter, onChange }: AgendaFiltersProps) {
  const id = useId();
  const modalityOptions: FilterOption[] = [{ value: 'all', label: copy.allModalities }, ...modalities];

  return (
    <div className="grid grid-cols-1 gap-3 sm:flex sm:flex-wrap">
      <FilterSelect
        id={`${id}-modality`}
        label={copy.modality}
        value={filter.modality}
        options={modalityOptions}
        onValueChange={(value) => {
          onChange({ ...filter, modality: modalityOf(value, modalities) });
        }}
      />
    </div>
  );
}
