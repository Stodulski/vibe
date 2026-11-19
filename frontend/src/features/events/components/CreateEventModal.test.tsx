import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { CreateEventModal } from './CreateEventModal';
import type { CourtWithPrices } from '@/shared/types/api.types';

vi.mock('../api/events.api', () => ({ eventsApi: { create: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));
const schedules = vi.hoisted(() => ({
  current: [] as { open_time: string; close_time: string; is_closed: boolean }[],
}));
vi.mock('@/features/complex', () => ({
  useSelectedComplex: () => ({ complex: { slug: 'club' } }),
  useSchedules: () => ({ data: schedules.current }),
}));

const t = ES_AR.events;
const modality = ES_AR.tournaments.traits.modality;

function court(id: string, name: string, isActive = true): CourtWithPrices {
  return {
    id,
    complex_id: 'c1',
    name,
    sport: 'padel',
    court_type: 'outdoor',
    is_active: isActive,
    has_lighting: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [],
  };
}

const COURTS = [court('ct1', 'Cancha 1'), court('ct2', 'Cancha 2'), court('ct3', 'Cancha vieja', false)];

type User = ReturnType<typeof userEvent.setup>;

/** The start and end selects are both "Desde" and "Hasta" in name only; the level range uses the same words without the requirement mark. */
async function pickTime(user: User, label: string, time: string) {
  await user.click(screen.getByRole('combobox', { name: `${label} (${ES_AR.common.requiredMarker})` }));
  await user.click(await screen.findByRole('option', { name: time }));
}

async function pickLastEnabledDay(user: User) {
  await user.click(screen.getByRole('button', { name: new RegExp(t.form.date) }));
  const grid = await screen.findByRole('grid');
  const days = within(grid)
    .getAllByRole('button')
    .filter((day) => !(day as HTMLButtonElement).disabled);
  const last = days[days.length - 1];
  if (!last) throw new Error('no selectable day');
  await user.click(last);
}

/** Every trait the form asks for, as the owner fills it: Caballeros, the whole day's categories, 19:00 to 21:00. */
async function fillValid(user: User) {
  await user.click(screen.getByRole('button', { name: modality.men }));
  await pickLastEnabledDay(user);
  await pickTime(user, t.form.startTime, '19:00');
  await pickTime(user, t.form.endTime, '21:00');
  await user.type(screen.getByLabelText(new RegExp(t.form.price)), '5000');
}

async function renderModal() {
  const { eventsApi } = await import('../api/events.api');
  const onClose = vi.fn();
  renderWithProviders(<CreateEventModal open onClose={onClose} complexId="c1" courts={COURTS} />);
  return { eventsApi, onClose, user: userEvent.setup() };
}

describe('CreateEventModal', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    schedules.current = [];
  });

  it('shows the three modalities as one choice, with none picked yet', async () => {
    await renderModal();

    const group = screen.getByRole('group', { name: t.form.modality });
    expect(
      within(group)
        .getAllByRole('button')
        .map((b) => b.textContent),
    ).toEqual([modality.men, modality.women, modality.mixed]);
    expect(
      within(group)
        .getAllByRole('button')
        .every((b) => b.getAttribute('aria-pressed') === 'false'),
    ).toBe(true);
  });

  it('offers only the hours the complex is open, both ends included', async () => {
    schedules.current = [{ open_time: '08:00', close_time: '23:00', is_closed: false }];
    const { user } = await renderModal();

    await user.click(screen.getByRole('combobox', { name: new RegExp(t.form.startTime) }));
    const options = (await screen.findAllByRole('option')).map((o) => o.textContent);
    expect(options[0]).toBe('08:00');
    expect(options.at(-1)).toBe('23:00');
    expect(options).not.toContain('07:30');
  });

  it('asks for the modality, date, hours and price when sent empty, and sends nothing', async () => {
    const { eventsApi, user } = await renderModal();

    await user.click(screen.getByRole('button', { name: t.form.submit }));

    expect(await screen.findByText(t.validation.modalityRequired)).toBeInTheDocument();
    expect(screen.getByText(ES_AR.validation.dateRequired)).toBeInTheDocument();
    expect(screen.getAllByText(ES_AR.validation.timeRequired)).toHaveLength(2);
    expect(screen.getByText(t.validation.priceRequired)).toBeInTheDocument();
    expect(eventsApi.create).not.toHaveBeenCalled();
  });
});

describe('CreateEventModal categories', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('starts with every category, and the range selects appear only once that is switched off', async () => {
    const { user } = await renderModal();

    expect(screen.getByRole('switch', { name: t.form.allLevels })).toBeChecked();
    expect(screen.queryByRole('combobox', { name: t.form.levelFrom })).not.toBeInTheDocument();

    await user.click(screen.getByRole('switch', { name: t.form.allLevels }));

    expect(screen.getByRole('combobox', { name: t.form.levelFrom })).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: t.form.levelTo })).toBeInTheDocument();
  });

  it('asks for both ends of the range once it is off, and does not send without them', async () => {
    const { eventsApi, user } = await renderModal();
    await user.click(screen.getByRole('switch', { name: t.form.allLevels }));
    await fillValid(user);

    await user.click(screen.getByRole('button', { name: t.form.submit }));

    expect(await screen.findByText(t.validation.levelRangeIncomplete)).toBeInTheDocument();
    expect(eventsApi.create).not.toHaveBeenCalled();
  });

  it('refuses a range whose "Desde" is weaker than its "Hasta"', async () => {
    const { eventsApi, user } = await renderModal();
    await user.click(screen.getByRole('switch', { name: t.form.allLevels }));
    await fillValid(user);
    await user.click(screen.getByRole('combobox', { name: t.form.levelFrom }));
    await user.click(await screen.findByRole('option', { name: '8va' }));
    await user.click(screen.getByRole('combobox', { name: t.form.levelTo }));
    await user.click(await screen.findByRole('option', { name: '6ta' }));

    await user.click(screen.getByRole('button', { name: t.form.submit }));

    expect(await screen.findByText(t.validation.levelRangeOrder)).toBeInTheDocument();
    expect(eventsApi.create).not.toHaveBeenCalled();
  });
});

// S15: off padel the modality is worded by the sport and no level range is offered.
describe('CreateEventModal by sport', () => {
  it('words the modality as masculino and offers no level range on soccer', async () => {
    const { user } = await renderModal();
    await user.click(screen.getByLabelText(new RegExp(t.form.sport)));
    await user.click(await screen.findByRole('option', { name: ES_AR.courts.sportTypes.soccer }));

    expect(screen.getByRole('button', { name: ES_AR.tournaments.traits.modalityMasculine.men })).toBeInTheDocument();
    expect(screen.queryByRole('switch', { name: t.form.allLevels })).not.toBeInTheDocument();
  });
});

describe('CreateEventModal submit', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('sends the modality, every court as an absent court_ids, and the price in centavos, and no name', async () => {
    const { eventsApi, onClose, user } = await renderModal();
    vi.mocked(eventsApi.create).mockResolvedValueOnce({ event: {} as never });

    await fillValid(user);
    await user.clear(screen.getByLabelText(new RegExp(t.form.capacity)));
    await user.type(screen.getByLabelText(new RegExp(t.form.capacity)), '24');
    await user.click(screen.getByRole('button', { name: t.form.submit }));

    await waitFor(() => {
      expect(eventsApi.create).toHaveBeenCalledTimes(1);
    });
    const body = vi.mocked(eventsApi.create).mock.calls[0]?.[1];
    expect(body).toMatchObject({
      modality: 'men',
      sport: 'padel',
      start_time: '19:00',
      end_time: '21:00',
      capacity: 24,
      price: 500000,
    });
    expect(body).not.toHaveProperty('name');
    expect(body).not.toHaveProperty('level_min');
    expect(body).not.toHaveProperty('court_ids');
    expect(body?.date).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    await waitFor(() => {
      expect(onClose).toHaveBeenCalled();
    });
  });

  it('sends the category range, strongest first, when it is set', async () => {
    const { eventsApi, user } = await renderModal();
    vi.mocked(eventsApi.create).mockResolvedValueOnce({ event: {} as never });
    await user.click(screen.getByRole('switch', { name: t.form.allLevels }));
    await fillValid(user);
    await user.click(screen.getByRole('combobox', { name: t.form.levelFrom }));
    await user.click(await screen.findByRole('option', { name: '6ta' }));
    await user.click(screen.getByRole('combobox', { name: t.form.levelTo }));
    await user.click(await screen.findByRole('option', { name: '8va' }));

    await user.click(screen.getByRole('button', { name: t.form.submit }));

    await waitFor(() => {
      expect(eventsApi.create).toHaveBeenCalledTimes(1);
    });
    expect(vi.mocked(eventsApi.create).mock.calls[0]?.[1]).toMatchObject({
      modality: 'men',
      level_min: 6,
      level_max: 8,
    });
  });
});

describe('CreateEventModal courts and hours', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('sends only the chosen courts, and never offers an inactive one', async () => {
    const { eventsApi, user } = await renderModal();
    vi.mocked(eventsApi.create).mockResolvedValueOnce({ event: {} as never });

    await user.click(screen.getByRole('checkbox', { name: t.form.allCourts }));
    expect(screen.queryByRole('checkbox', { name: 'Cancha vieja' })).not.toBeInTheDocument();
    await user.click(screen.getByRole('checkbox', { name: 'Cancha 2' }));
    await fillValid(user);
    await user.click(screen.getByRole('button', { name: t.form.submit }));

    await waitFor(() => {
      expect(eventsApi.create).toHaveBeenCalledTimes(1);
    });
    expect(vi.mocked(eventsApi.create).mock.calls[0]?.[1]).toMatchObject({ court_ids: ['ct2'] });
  });

  it('asks for at least one court once "Todas las canchas" is off', async () => {
    const { eventsApi, user } = await renderModal();

    await user.click(screen.getByRole('checkbox', { name: t.form.allCourts }));
    await fillValid(user);
    await user.click(screen.getByRole('button', { name: t.form.submit }));

    expect(await screen.findByText(t.validation.courtsRequired)).toBeInTheDocument();
    expect(eventsApi.create).not.toHaveBeenCalled();
  });

  it('refuses an event that would last more than 12 hours', async () => {
    const { eventsApi, user } = await renderModal();

    await fillValid(user);
    await pickTime(user, t.form.endTime, '19:00');
    await user.click(screen.getByRole('button', { name: t.form.submit }));

    // An end at the start again reads as the next day: 24 hours.
    expect(await screen.findByText(t.validation.duration)).toBeInTheDocument();
    expect(eventsApi.create).not.toHaveBeenCalled();
  });
});

describe('CreateEventModal failures', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('says in Spanish what holds the hours when the server answers 409, and keeps the form open', async () => {
    const { eventsApi, onClose, user } = await renderModal();
    vi.mocked(eventsApi.create).mockRejectedValueOnce(
      await makeConsumedHttpError(409, {
        title: 'Conflict',
        detail: 'a booking already holds those hours on Cancha 2',
      }),
    );

    await fillValid(user);
    await user.click(screen.getByRole('button', { name: t.form.submit }));

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(t.conflict.title);
    expect(alert).toHaveTextContent(t.conflict.booking('Cancha 2'));
    expect(alert).not.toHaveTextContent('holds those hours');
    expect(onClose).not.toHaveBeenCalled();
    expect(screen.getByLabelText(new RegExp(t.form.capacity))).toHaveValue(20);
  });

  it('shows the server message for any other failure', async () => {
    const { eventsApi, user } = await renderModal();
    vi.mocked(eventsApi.create).mockRejectedValueOnce(
      await makeConsumedHttpError(400, { title: 'Bad Request', detail: 'La fecha ya pasó' }),
    );

    await fillValid(user);
    await user.click(screen.getByRole('button', { name: t.form.submit }));

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(t.form.createError);
    expect(alert).toHaveTextContent('La fecha ya pasó');
  });
});
