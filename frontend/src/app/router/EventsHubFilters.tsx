import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { AgendaDayOption, AgendaFilter, AgendaModalityOption, AgendaTab } from './eventsHubAgenda';
import { EVENTS_FILTER_BAR_CLASS, EVENTS_FILTER_FIELD_CLASS } from './eventsHubLayout';

const t = ES_AR.events.filters;

interface FilterOption {
  value: string;
  label: string;
}

/** The window the list loads, Próximos first. */
const WHEN_OPTIONS: FilterOption[] = [
  { value: 'upcoming', label: ES_AR.events.upcoming },
  { value: 'past', label: ES_AR.events.past },
];

const KIND_OPTIONS: FilterOption[] = [
  { value: 'all', label: t.allKinds },
  { value: 'openPlay', label: t.kindOpenPlay },
  { value: 'tournament', label: t.kindTournament },
];

/** A label over one select, the way the reports bar builds its pickers. */
function FilterField({
  id,
  label,
  value,
  options,
  onValueChange,
}: {
  id: string;
  label: string;
  value: string;
  options: FilterOption[];
  onValueChange: (value: string) => void;
}) {
  return (
    <div className={EVENTS_FILTER_FIELD_CLASS}>
      <Label htmlFor={id}>{label}</Label>
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
    </div>
  );
}

/** The window a chosen value stands for; anything that is not Pasados is Próximos. */
function whenOf(value: string): AgendaTab {
  return value === 'past' ? 'past' : 'upcoming';
}

/** The kind a chosen value stands for; anything that is not a kind is "all". */
function kindOf(value: string): AgendaFilter['kind'] {
  return value === 'openPlay' || value === 'tournament' ? value : 'all';
}

interface EventsHubFiltersProps {
  /** The window the list loads. Choosing the other one clears the filters below. */
  tab: AgendaTab;
  onTabChange: (tab: AgendaTab) => void;
  filter: AgendaFilter;
  onChange: (filter: AgendaFilter) => void;
  /** The modalities the loaded rows have; "all" is always offered first. */
  modalities: AgendaModalityOption[];
  /** The days the loaded rows start on; every day is always offered first. */
  days: AgendaDayOption[];
}

/**
 * The window, kind, modality and day of the events list, as the app's own selects with their
 * labels above them. Kind, modality and day filter the rows already loaded, so changing one
 * never fetches; the window loads the other period.
 */
export function EventsHubFilters({ tab, onTabChange, filter, onChange, modalities, days }: EventsHubFiltersProps) {
  const modalityOptions: FilterOption[] = [
    { value: 'all', label: t.allModalities },
    ...modalities.map((option) => ({ value: option.value, label: option.label })),
  ];
  const dayOptions: FilterOption[] = [
    { value: 'all', label: t.allDays },
    ...days.map((option) => ({ value: option.date, label: option.label })),
  ];

  return (
    <div className={EVENTS_FILTER_BAR_CLASS}>
      <FilterField
        id="events-filter-when"
        label={t.when}
        value={tab}
        options={WHEN_OPTIONS}
        onValueChange={(value) => {
          onTabChange(whenOf(value));
        }}
      />
      <FilterField
        id="events-filter-kind"
        label={t.kind}
        value={filter.kind}
        options={KIND_OPTIONS}
        onValueChange={(value) => {
          onChange({ ...filter, kind: kindOf(value) });
        }}
      />
      <FilterField
        id="events-filter-modality"
        label={t.modality}
        value={filter.modality}
        options={modalityOptions}
        onValueChange={(value) => {
          const chosen = modalities.find((option) => option.value === value);
          onChange({ ...filter, modality: chosen?.value ?? 'all' });
        }}
      />
      <FilterField
        id="events-filter-day"
        label={t.day}
        value={filter.day ?? 'all'}
        options={dayOptions}
        onValueChange={(value) => {
          onChange({ ...filter, day: value === 'all' ? null : value });
        }}
      />
    </div>
  );
}
