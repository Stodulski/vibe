import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor, within } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { EventDetail } from './EventDetail';
import type { OpenPlayEvent, OpenPlayParticipant } from '@/shared/types/api.types';

vi.mock('../api/events.api', () => ({ eventsApi: { get: vi.fn(), withdraw: vi.fn() } }));
vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }));

const t = ES_AR.events;

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
  courts: [{ id: 'ct1', name: 'Cancha 1' }],
  confirmed_count: 1,
  held_count: 1,
  spots_left: 18,
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
  participant({ id: 'r2', first_name: 'Beto', last_name: 'Diaz', status: 'held', payment: null }),
  participant({ id: 'r3', first_name: 'Carla', last_name: 'Ruiz', status: 'cancelled', cancelled_by: 'participant' }),
];

async function renderDetail(event: OpenPlayEvent = EVENT) {
  const { eventsApi } = await import('../api/events.api');
  vi.mocked(eventsApi.get).mockResolvedValue({ event, participants: PARTICIPANTS });
  renderWithProviders(<EventDetail open onClose={vi.fn()} complexId="c1" eventId="ev1" />);
  const list = await screen.findByRole('list', { name: t.detail.participants });
  return { eventsApi, list, user: userEvent.setup() };
}

const menuOf = (name: string) => screen.queryByRole('button', { name: t.withdraw.actions(name) });

describe('EventDetail: withdrawing a participant', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('says a paid participant gets everything back, and withdraws them', async () => {
    const { eventsApi, user } = await renderDetail();
    vi.mocked(eventsApi.withdraw).mockResolvedValue({
      registration_id: 'r1',
      refund: { status: 'issued', message: 'ok', amount: 520000 },
    });

    const menu = menuOf('Ana Gomez');
    if (!menu) throw new Error('no menu for Ana');
    await user.click(menu);
    await user.click(await screen.findByRole('menuitem', { name: t.withdraw.action }));
    const dialog = await screen.findByRole('alertdialog');
    expect(dialog).toHaveTextContent(t.withdraw.confirmDescriptionPaid);
    await user.click(within(dialog).getByRole('button', { name: t.withdraw.confirmLabel }));

    await waitFor(() => {
      expect(eventsApi.withdraw).toHaveBeenCalledWith('c1', 'ev1', 'r1');
    });
  });

  it('says a participant still paying has nothing to get back', async () => {
    const { user } = await renderDetail();

    const menu = menuOf('Beto Diaz');
    if (!menu) throw new Error('no menu for Beto');
    await user.click(menu);
    await user.click(await screen.findByRole('menuitem', { name: t.withdraw.action }));
    expect(await screen.findByRole('alertdialog')).toHaveTextContent(t.withdraw.confirmDescriptionUnpaid);
  });

  it('offers no withdrawal for a spot that is already over', async () => {
    await renderDetail();
    expect(menuOf('Carla Ruiz')).not.toBeInTheDocument();
  });

  it('offers withdrawals a minute before the event starts', async () => {
    // 18:59 in Argentina (UTC-3) on the event's day; the next case is 19:01.
    vi.useFakeTimers({ toFake: ['Date'], now: new Date('2099-01-02T21:59:00Z') });
    await renderDetail();
    expect(menuOf('Ana Gomez')).toBeInTheDocument();
  });

  it('offers none once the event has started', async () => {
    vi.useFakeTimers({ toFake: ['Date'], now: new Date('2099-01-02T22:01:00Z') });
    await renderDetail();
    expect(menuOf('Ana Gomez')).not.toBeInTheDocument();
  });
});
