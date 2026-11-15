import { describe, it, expect, vi, beforeEach } from 'vitest';
import { toast } from 'sonner';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { EventDetail } from './EventDetail';
import type { OpenPlayEvent, OpenPlayParticipant } from '@/shared/types/api.types';

vi.mock('../api/events.api', () => ({ eventsApi: { get: vi.fn(), cancel: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.events;

// A Friday far ahead, so the event is never in the past.
const EVENT: OpenPlayEvent = {
  id: 'ev1',
  complex_id: 'c1',
  name: 'Americano de viernes',
  sport: 'padel',
  date: '2099-01-02',
  start_time: '19:00',
  end_time: '21:00',
  duration_minutes: 120,
  capacity: 20,
  price: 500000,
  status: 'active',
  courts: [
    { id: 'ct1', name: 'Cancha 1' },
    { id: 'ct2', name: 'Cancha 2' },
  ],
  confirmed_count: 2,
  held_count: 1,
  spots_left: 17,
  created_at: '2026-11-14T12:00:00Z',
};

function participant(overrides: Partial<OpenPlayParticipant>): OpenPlayParticipant {
  return {
    id: 'r1',
    first_name: 'Ana',
    last_name: 'Gomez',
    phone: '+5491155550001',
    status: 'confirmed',
    price: 500000,
    service_fee: 20000,
    created_at: '2026-11-14T12:00:00Z',
    payment: { status: 'deposit_paid', amount: 520000, service_fee: 20000, refund_amount: 0, method: 'mercadopago' },
    ...overrides,
  };
}

const PARTICIPANTS = [
  participant({ id: 'r1' }),
  participant({
    id: 'r2',
    first_name: 'Beto',
    last_name: 'Diaz',
    phone: '+5491155550002',
    status: 'held',
    payment: null,
  }),
  participant({
    id: 'r3',
    first_name: 'Carla',
    last_name: 'Ruiz',
    phone: '+5491155550003',
    status: 'cancelled',
    cancelled_by: 'participant',
    payment: { status: 'refunded', amount: 520000, service_fee: 20000, refund_amount: 520000, method: 'mercadopago' },
  }),
];

async function renderDetail(event: OpenPlayEvent = EVENT, participants: OpenPlayParticipant[] = PARTICIPANTS) {
  const { eventsApi } = await import('../api/events.api');
  vi.mocked(eventsApi.get).mockResolvedValue({ event, participants });
  renderWithProviders(<EventDetail open onClose={vi.fn()} complexId="c1" eventId="ev1" />);
  return { eventsApi, user: userEvent.setup() };
}

/** The cancel is a visible button in the pinned footer. */
async function findCancelButton() {
  return screen.findByRole('button', { name: t.cancel.action });
}

describe('EventDetail', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows the event facts', async () => {
    await renderDetail();

    expect(await screen.findByText('Americano de viernes')).toBeInTheDocument();
    expect(screen.getByText('Cancha 1, Cancha 2')).toBeInTheDocument();
    expect(screen.getByText('19:00 a 21:00')).toBeInTheDocument();
    expect(screen.getByText(t.spotsTaken(2, 20))).toBeInTheDocument();
  });

  it('lists every participant with the state of their payment', async () => {
    await renderDetail();

    const list = within(await screen.findByRole('list', { name: t.detail.participants }));
    const rows = list.getAllByRole('listitem');
    expect(rows).toHaveLength(3);
    expect(rows[0]).toHaveTextContent('Ana Gomez');
    expect(rows[0]).toHaveTextContent(t.participant.confirmed);
    expect(rows[1]).toHaveTextContent('Beto Diaz');
    expect(rows[1]).toHaveTextContent(t.participant.held);
    expect(rows[2]).toHaveTextContent('Carla Ruiz');
    expect(rows[2]).toHaveTextContent(t.participant.refunded);
    expect(rows[2]).toHaveTextContent(t.participant.cancelledByParticipant);
    expect(rows[0]).toHaveTextContent(t.participant.paid('$5.200'));
  });

  it('shows a spot that was only held and then cancelled by the owner as unpaid, not as a cancellation', async () => {
    const unpaid = participant({
      id: 'r4',
      first_name: 'Dani',
      last_name: 'Paz',
      phone: '+5491155550004',
      status: 'cancelled',
      cancelled_by: 'owner',
      payment: null,
    });
    await renderDetail({ ...EVENT, status: 'cancelled', cancelled_at: '2026-11-14T15:00:00Z' }, [unpaid]);

    const row = within(await screen.findByRole('list', { name: t.detail.participants })).getByRole('listitem');
    expect(row).toHaveTextContent(t.participant.cancelledUnpaid);
    expect(row).toHaveTextContent(t.participant.cancelledByOwner);
    expect(row).not.toHaveTextContent('Canceló');
  });
});

describe('EventDetail — header and participants', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('counts only the confirmed people in the heading', async () => {
    await renderDetail();

    expect(await screen.findByText(`${t.detail.participants} · 1 confirmado`)).toBeInTheDocument();
  });

  it('puts the date under the name and writes it in capitals like the other panels', async () => {
    await renderDetail();

    const subtitle = await screen.findByText('viernes 2 de enero', { selector: 'p span' });
    expect(subtitle).toHaveClass('inline-block');
  });

  it('keeps the confirmed people first and the cancelled ones last', async () => {
    await renderDetail(EVENT, [...PARTICIPANTS].reverse());

    const rows = within(await screen.findByRole('list', { name: t.detail.participants })).getAllByRole('listitem');
    expect(rows.map((row) => row.textContent)).toEqual([
      expect.stringContaining('Ana'),
      expect.stringContaining('Beto'),
      expect.stringContaining('Carla'),
    ]);
  });

  it('says so when nobody signed up yet', async () => {
    await renderDetail(EVENT, []);
    expect(await screen.findByText(t.detail.noParticipants)).toBeInTheDocument();
  });
});

describe('EventDetail cancel dialog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('says it in two short sentences, and confirms with "Sí, cancelar evento"', async () => {
    const { user } = await renderDetail();

    await user.click(await findCancelButton());
    const dialog = await screen.findByRole('alertdialog');

    const sentences = t.cancel.confirmDescriptionPaid(3).split(/(?<=\.)\s+/);
    expect(sentences).toHaveLength(2);
    for (const sentence of sentences) expect(sentence.split(/\s+/).length).toBeLessThanOrEqual(25);
    expect(within(dialog).getByRole('button', { name: 'Sí, cancelar evento' })).toBeInTheDocument();
    expect(within(dialog).getByRole('button', { name: 'Volver' })).toBeInTheDocument();
  });

  it('shows the cancel as a destructive outline button in the footer, with no overflow menu', async () => {
    await renderDetail();

    const button = await findCancelButton();
    expect(button).toHaveAttribute('data-variant', 'destructive-outline');
    expect(button.closest('[data-slot="sheet-footer"]')).not.toBeNull();
    expect(screen.queryByRole('button', { name: ES_AR.common.rowActionsLabel })).not.toBeInTheDocument();
  });
});

describe('EventDetail cancel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('explains that every paid participant is refunded, counting them, before cancelling', async () => {
    const { user } = await renderDetail();

    await user.click(await findCancelButton());

    expect(await screen.findByText(t.cancel.confirmDescriptionPaid(1))).toBeInTheDocument();
  });

  it('cancels the event on confirm and reports the refunds on their way', async () => {
    const { eventsApi, user } = await renderDetail();
    vi.mocked(eventsApi.cancel).mockResolvedValueOnce({
      event: { ...EVENT, status: 'cancelled' },
      refunds: [{ registration_id: 'r1', refund: { status: 'issued', message: 'ok', amount: 520000 } }],
    });

    await user.click(await findCancelButton());
    const dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: /cancelar evento/i }));

    await waitFor(() => {
      expect(eventsApi.cancel).toHaveBeenCalledWith('c1', 'ev1');
    });
    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(t.cancel.success(1));
    });
  });

  it('keeps the event when the server refuses the cancel, and says why', async () => {
    const { eventsApi, user } = await renderDetail();
    const { makeConsumedHttpError } = await import('@/test/factories');
    vi.mocked(eventsApi.cancel).mockRejectedValueOnce(
      await makeConsumedHttpError(400, { title: 'Bad Request', detail: 'el evento ya está cancelado' }),
    );

    await user.click(await findCancelButton());
    const dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: /cancelar evento/i }));

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith(t.cancel.error, { description: 'el evento ya está cancelado' });
    });
  });

  it('offers no cancel for an event that is already cancelled', async () => {
    await renderDetail({ ...EVENT, status: 'cancelled', cancelled_at: '2026-11-14T15:00:00Z' });

    expect(await screen.findByText(new RegExp(t.detail.cancelledNote))).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: t.cancel.action })).not.toBeInTheDocument();
  });

  it('offers no cancel for an event whose day is over', async () => {
    await renderDetail({ ...EVENT, date: '2020-01-02' });

    await screen.findByText('Americano de viernes');
    expect(screen.queryByRole('button', { name: t.cancel.action })).not.toBeInTheDocument();
  });
});
