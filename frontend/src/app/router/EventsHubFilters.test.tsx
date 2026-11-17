import { describe, it, expect, vi } from 'vitest';
import { renderWithProviders as render, screen, userEvent } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import {
  ALL_AGENDA_FILTER,
  type AgendaDayOption,
  type AgendaFilter,
  type AgendaModalityOption,
  type AgendaTab,
} from './eventsHubAgenda';
import { EventsHubFilters } from './EventsHubFilters';

const t = ES_AR.events.filters;

const MODALITIES: AgendaModalityOption[] = [
  { value: 'men', label: 'Caballeros' },
  { value: 'women', label: 'Damas' },
];
const DAYS: AgendaDayOption[] = [{ date: '2026-11-18', label: 'Mié 18 nov' }];

interface RenderOptions {
  tab?: AgendaTab;
  filter?: AgendaFilter;
}

/** Renders the bar and returns the handlers the tests assert on. */
function renderFilters({ tab = 'upcoming', filter = ALL_AGENDA_FILTER }: RenderOptions = {}) {
  const onChange = vi.fn();
  const onTabChange = vi.fn();
  render(
    <EventsHubFilters
      tab={tab}
      onTabChange={onTabChange}
      filter={filter}
      onChange={onChange}
      modalities={MODALITIES}
      days={DAYS}
    />,
  );
  return { onChange, onTabChange };
}

describe('EventsHubFilters', () => {
  it('labels the four selects in order, the window first, each showing its first choice', () => {
    renderFilters();

    const ids = Array.from(document.querySelectorAll('[id^="events-filter-"]')).map((el) => el.id);
    expect(ids).toEqual(['events-filter-when', 'events-filter-kind', 'events-filter-modality', 'events-filter-day']);
    expect(screen.getByLabelText(t.when)).toHaveTextContent(ES_AR.events.upcoming);
    expect(screen.getByLabelText(t.kind)).toHaveTextContent(t.allKinds);
    expect(screen.getByLabelText(t.modality)).toHaveTextContent(t.allModalities);
    expect(screen.getByLabelText(t.day)).toHaveTextContent(t.allDays);
  });

  it('reports the window chosen, and leaves the filters alone', async () => {
    const { onChange, onTabChange } = renderFilters({ filter: { ...ALL_AGENDA_FILTER, kind: 'tournament' } });
    const user = userEvent.setup();

    await user.click(screen.getByLabelText(t.when));
    await user.click(await screen.findByRole('option', { name: ES_AR.events.past }));

    expect(onTabChange).toHaveBeenCalledWith('past');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('reports the kind chosen and keeps the other filters as they are', async () => {
    const { onChange } = renderFilters({ filter: { ...ALL_AGENDA_FILTER, modality: 'women' } });
    const user = userEvent.setup();

    await user.click(screen.getByLabelText(t.kind));
    await user.click(await screen.findByRole('option', { name: t.kindTournament }));

    expect(onChange).toHaveBeenCalledWith({ kind: 'tournament', modality: 'women', day: null });
  });

  it('reports the modality chosen, offering the modalities the rows have after "all"', async () => {
    const { onChange } = renderFilters();
    const user = userEvent.setup();

    await user.click(screen.getByLabelText(t.modality));
    await user.click(await screen.findByRole('option', { name: 'Damas' }));

    expect(onChange).toHaveBeenCalledWith({ kind: 'all', modality: 'women', day: null });
  });

  it('reports the day chosen, and clears it back to every day with "Todos"', async () => {
    const { onChange } = renderFilters({ filter: { ...ALL_AGENDA_FILTER, day: '2026-11-18' } });
    const user = userEvent.setup();

    await user.click(screen.getByLabelText(t.day));
    await user.click(await screen.findByRole('option', { name: t.allDays }));

    expect(onChange).toHaveBeenCalledWith({ kind: 'all', modality: 'all', day: null });
  });
});
