import { describe, it, expect, vi, beforeEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import userEvent from '@testing-library/user-event';
import { toast } from 'sonner';
import { renderWithProviders, screen, waitFor, within } from '@/test/test-utils';
import { server } from '@/test/msw/server';
import { ES_AR } from '@/shared/i18n/es_AR';
import { InstagramConnectCard } from '../InstagramConnectCard';

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.instagram.stories;

const DEFAULTS = { daily_enabled: false, daily_time: '10:00', freed_enabled: false, freed_daily_cap: 5 };

function connection(overrides: { connected: boolean; needs_reconnect: boolean }) {
  server.use(
    http.get('*/complexes/:complexId/instagram', () =>
      HttpResponse.json({
        enabled: true,
        username: 'club.padel',
        account_type: 'BUSINESS',
        expires_at: null,
        ...overrides,
      }),
    ),
  );
}

/** Records every PUT body; `respond` lets a test refuse it. */
function capturePut(respond?: () => Response) {
  const bodies: unknown[] = [];
  server.use(
    http.put('*/complexes/:complexId/instagram/stories', async ({ request }) => {
      const body = await request.json();
      bodies.push(body);
      return respond ? respond() : HttpResponse.json({ settings: body });
    }),
  );
  return bodies;
}

/** Serves the given recent stories (freed, scheduled 18:00 venue time) under the default settings. */
function recent(stories: { status: string; reason: string | null }[]) {
  server.use(
    http.get('*/complexes/:complexId/instagram/stories', () =>
      HttpResponse.json({
        settings: DEFAULTS,
        recent: stories.map((s) => ({
          kind: 'freed',
          scheduled_for: '2026-11-19T21:00:00Z',
          published_at: null,
          ...s,
        })),
      }),
    ),
  );
}

function renderCard() {
  renderWithProviders(<InstagramConnectCard complexId="c1" slug="los-alamos" />);
}

const saveButton = () => screen.findByRole('button', { name: t.save });

describe('InstagramStoriesSection visibility', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it.each([
    ['not connected', { connected: false, needs_reconnect: false }],
    ['needs reconnect', { connected: false, needs_reconnect: true }],
  ])('is not offered when %s, and does not ask for the stories', async (_name, state) => {
    const read = vi.fn();
    server.use(
      http.get('*/complexes/:complexId/instagram/stories', () => {
        read();
        return HttpResponse.json({ settings: DEFAULTS, recent: [] });
      }),
    );
    connection(state);
    renderCard();

    expect(await screen.findByText(ES_AR.instagram.professionalNote)).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: t.title })).not.toBeInTheDocument();
    expect(read).not.toHaveBeenCalled();
  });

  it('links the preview to the landing daily story image of this complex, in a new tab', async () => {
    connection({ connected: true, needs_reconnect: false });
    renderCard();

    const link = await screen.findByRole('link', { name: t.preview });
    expect(link).toHaveAttribute('href', 'https://vibe.com.ar/historias/c/los-alamos/libres.jpg?vista=1');
    expect(link).toHaveAttribute('target', '_blank');
  });
});

describe('InstagramStoriesSection saving', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    connection({ connected: true, needs_reconnect: false });
  });

  it('keeps save disabled until something changes, then sends exactly the edited settings', async () => {
    const user = userEvent.setup();
    const bodies = capturePut();
    renderCard();

    expect(await saveButton()).toBeDisabled();
    await user.selectOptions(screen.getByRole('combobox', { name: t.dailyTime }), '18:00');
    await user.click(screen.getByRole('combobox', { name: t.freedCap }));
    await user.click(await screen.findByRole('option', { name: t.capOption(3) }));
    await user.click(await saveButton());

    await waitFor(() => {
      expect(bodies).toEqual([{ ...DEFAULTS, daily_time: '18:00', freed_daily_cap: 3 }]);
    });
    expect(toast.success).toHaveBeenCalledWith(t.saved);
    await waitFor(async () => {
      expect(await saveButton()).toBeDisabled();
    });
  });

  it('sends true for a type the owner turns on', async () => {
    const user = userEvent.setup();
    const bodies = capturePut();
    renderCard();

    await user.click(await screen.findByRole('switch', { name: t.freed }));
    await user.click(await saveButton());

    await waitFor(() => {
      expect(bodies).toEqual([{ ...DEFAULTS, freed_enabled: true }]);
    });
  });

  it('offers only the hours a daily story may go out, 08:00 to 22:00', async () => {
    renderCard();

    const select = await screen.findByRole('combobox', { name: t.dailyTime });
    const hours = within(select)
      .getAllByRole('option')
      .map((option) => option.getAttribute('value'));
    expect(hours[0]).toBe('08:00');
    expect(hours.at(-1)).toBe('22:00');
  });
});

describe('InstagramStoriesSection server refusal and history', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    connection({ connected: true, needs_reconnect: false });
  });

  it('shows a 422 on the field it names and keeps what the owner chose', async () => {
    const user = userEvent.setup();
    capturePut(() =>
      HttpResponse.json(
        { title: 'Validation Failed', errors: [{ field: 'daily_time', message: 'Horario fuera de rango' }] },
        { status: 422 },
      ),
    );
    renderCard();

    await user.click(await screen.findByRole('switch', { name: t.daily }));
    await user.selectOptions(screen.getByRole('combobox', { name: t.dailyTime }), '21:00');
    await user.click(await saveButton());

    expect(await screen.findByText('Horario fuera de rango')).toBeInTheDocument();
    expect(screen.getByRole('combobox', { name: t.dailyTime })).toHaveValue('21:00');
    expect(screen.getByRole('switch', { name: t.daily })).toBeChecked();
    expect(toast.success).not.toHaveBeenCalled();
  });

  it('lists the recent stories with their status, or says there are none', async () => {
    server.use(
      http.get('*/complexes/:complexId/instagram/stories', () =>
        HttpResponse.json({
          settings: DEFAULTS,
          recent: [
            { kind: 'freed', status: 'failed', scheduled_for: '2026-11-19T21:00:00Z', published_at: null, reason: 'x' },
            {
              kind: 'daily',
              status: 'published',
              scheduled_for: '2026-11-19T13:00:00Z',
              published_at: '2026-11-19T13:01:00Z',
              reason: null,
            },
          ],
        }),
      ),
    );
    renderCard();

    const items = await screen.findAllByRole('listitem');
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent(t.statuses.failed);
    expect(items[1]).toHaveTextContent(t.statuses.published);
    expect(items[1]).toHaveTextContent('10:01');
    expect(screen.queryByText(t.recentEmpty)).not.toBeInTheDocument();
  });

  it('says there are no stories yet when the list is empty', async () => {
    renderCard();
    expect(await screen.findByText(t.recentEmpty)).toBeInTheDocument();
  });
});

describe('InstagramStoriesSection skip and failure explanations', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    connection({ connected: true, needs_reconnect: false });
  });

  it.each([
    ['nothing_to_show', 'no quedaban turnos libres'],
    ['disabled', 'la apagaste'],
    ['account_unusable', 'Instagram desconectado'],
    ['day_passed', 'ya pasó el día'],
    ['too_close', 'faltaba menos de una hora'],
  ])('explains a story skipped for %s', async (reason, explanation) => {
    recent([{ status: 'skipped', reason }]);
    renderCard();

    const [item] = await screen.findAllByRole('listitem');
    expect(item).toHaveTextContent(t.statuses.skipped);
    expect(item).toHaveTextContent(explanation);
  });

  it('shows a skipped story with an unknown or missing reason as plainly skipped', async () => {
    recent([
      { status: 'skipped', reason: 'some_new_code' },
      { status: 'skipped', reason: null },
    ]);
    renderCard();

    const items = await screen.findAllByRole('listitem');
    expect(items).toHaveLength(2);
    for (const item of items) {
      expect(item).toHaveTextContent('Omitida');
      expect(item).not.toHaveTextContent('some_new_code');
      for (const explanation of Object.values(t.skipReasons)) expect(item).not.toHaveTextContent(explanation);
    }
  });

  it('shows a failed story with a generic cause, never the technical reason', async () => {
    recent([{ status: 'failed', reason: 'graph 400: (#100) Invalid parameter' }]);
    renderCard();

    const [item] = await screen.findAllByRole('listitem');
    expect(item).toHaveTextContent('Falló');
    expect(item).toHaveTextContent('Instagram no la aceptó');
    expect(item).not.toHaveTextContent('Invalid parameter');
  });
});
