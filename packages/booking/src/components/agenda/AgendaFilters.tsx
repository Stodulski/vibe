import { useId } from 'react';
import { FormField, Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@vibe/ui';
import { type AgendaFilter, type DayChip, type ModalityOption } from './agendaModel';

/** The words of the two filters: their labels and the "all" option of each. */
export interface AgendaFilterCopy {
  modality: string;
  day: string;
  allModalities: string;
  allDays: string;
}

interface AgendaFiltersProps {
  copy: AgendaFilterCopy;
  /** The modalities the rows have; "all" is always offered first. */
  modalities: readonly ModalityOption[];
  /** The days the rows fall on; every day is always offered first. */
  days: readonly DayChip[];
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
 * The modality and the day of an agenda, as two selects with their labels above
 * them: side by side at 50% each on a phone, and side by side at a compact width
 * on a desktop.
 */
export function AgendaFilters({ copy, modalities, days, filter, onChange }: AgendaFiltersProps) {
  const id = useId();
  const modalityOptions: FilterOption[] = [{ value: 'all', label: copy.allModalities }, ...modalities];
  const dayOptions: FilterOption[] = [
    { value: 'all', label: copy.allDays },
    ...days.map((chip) => ({ value: chip.date, label: chip.label })),
  ];

  return (
    <div className="grid grid-cols-2 gap-3 sm:flex sm:flex-wrap">
      <FilterSelect
        id={`${id}-modality`}
        label={copy.modality}
        value={filter.modality}
        options={modalityOptions}
        onValueChange={(value) => {
          onChange({ ...filter, modality: modalityOf(value, modalities) });
        }}
      />
      <FilterSelect
        id={`${id}-day`}
        label={copy.day}
        value={filter.day ?? 'all'}
        options={dayOptions}
        onValueChange={(value) => {
          onChange({ ...filter, day: value === 'all' ? null : value });
        }}
      />
    </div>
  );
}
