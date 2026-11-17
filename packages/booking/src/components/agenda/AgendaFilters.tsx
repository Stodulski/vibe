import { useId } from 'react';
import { FormField } from '@vibe/ui';
import { SELECT_CLASS } from '../selectStyles';
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

/**
 * The modality and the day of an agenda, as two native selects with their labels
 * above them: side by side at 50% each on a phone, and side by side at a compact
 * width on a desktop.
 */
export function AgendaFilters({ copy, modalities, days, filter, onChange }: AgendaFiltersProps) {
  const id = useId();
  const modalityId = `${id}-modality`;
  const dayId = `${id}-day`;

  return (
    <div className="grid grid-cols-2 gap-3 sm:flex sm:flex-wrap">
      <div className="min-w-0 sm:w-60">
        <FormField label={copy.modality} htmlFor={modalityId}>
          <select
            id={modalityId}
            className={SELECT_CLASS}
            value={filter.modality}
            onChange={(event) => {
              onChange({ ...filter, modality: modalityOf(event.target.value, modalities) });
            }}
          >
            <option value="all">{copy.allModalities}</option>
            {modalities.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </FormField>
      </div>

      <div className="min-w-0 sm:w-60">
        <FormField label={copy.day} htmlFor={dayId}>
          <select
            id={dayId}
            className={SELECT_CLASS}
            value={filter.day ?? 'all'}
            onChange={(event) => {
              onChange({ ...filter, day: event.target.value === 'all' ? null : event.target.value });
            }}
          >
            <option value="all">{copy.allDays}</option>
            {days.map((chip) => (
              <option key={chip.date} value={chip.date}>
                {chip.label}
              </option>
            ))}
          </select>
        </FormField>
      </div>
    </div>
  );
}
