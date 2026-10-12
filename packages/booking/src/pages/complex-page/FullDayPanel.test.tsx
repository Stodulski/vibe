import { describe, it, expect, vi, beforeEach } from 'vitest';
import { http, HttpResponse } from 'msw';
import { screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { format } from 'date-fns/format';
import { server } from '../../test/msw/server';
import { renderBooking } from '../../test/booking';
import { BOOKING_MESSAGES } from '../../messages';
import { STORAGE_KEY } from '../../components/booking-form/savedFormData';
import { FullDayPanel } from './FullDayPanel';

const t = BOOKING_MESSAGES;

const SELECTED_DAY = '2026-10-14';
const NEXT_DAY = '2026-10-17';
const NEXT_DAY_LABEL = 'sábado 17/10';

function renderPanel(overrides: Partial<Parameters<typeof FullDayPanel>[0]> = {}) {
  const props = {
    slug: 'club-norte',
    date: SELECTED_DAY,
    duration: 90 as const,
    phone: '+5491155550000',
    onSelectDate: vi.fn(),
    ...overrides,
  };
  renderBooking(<FullDayPanel {...props} />);
  return props;
}

function nextAvailable(date: string | null) {
  server.use(http.get('*/public/complexes/:slug/next-available', () => HttpResponse.json({ date })));
}

function waitlistRespondsWith(response: () => Response | Promise<Response>) {
  server.use(http.post('*/public/complexes/:slug/waitlist', response));
}

function emailField() {
  return screen.getByLabelText(t.publicBooking.waitlistEmailLabel);
}

function submitWaitlist() {
  return userEvent.click(screen.getByRole('button', { name: t.publicBooking.waitlistSubmit }));
}

const confirmation = `${t.publicBooking.waitlistSuccessPrefix} ana@example.com ${t.publicBooking.waitlistSuccessSuffix}`;

describe('FullDayPanel — the next free day', () => {
  beforeEach(() => {
    nextAvailable(NEXT_DAY);
  });

  it('says the day is full and offers the next free day, which selects it', async () => {
    const onSelectDate = vi.fn();
    renderPanel({ onSelectDate });

    expect(await screen.findByText(t.publicBooking.fullDayTitle)).toBeInTheDocument();
    expect(await screen.findByText(`${t.publicBooking.nextAvailableDay} ${NEXT_DAY_LABEL}`)).toBeInTheDocument();

    await userEvent.click(
      await screen.findByRole('button', { name: `${t.publicBooking.viewNextAvailableDay} ${NEXT_DAY_LABEL}` }),
    );

    expect(onSelectDate).toHaveBeenCalledTimes(1);
    const [selected] = onSelectDate.mock.calls[0] as [Date];
    expect(format(selected, 'yyyy-MM-dd')).toBe(NEXT_DAY);
  });

  it('asks the next day with the chosen duration', async () => {
    let receivedDuration: string | null = null;
    server.use(
      http.get('*/public/complexes/:slug/next-available', ({ request }) => {
        receivedDuration = new URL(request.url).searchParams.get('duration');
        return HttpResponse.json({ date: NEXT_DAY });
      }),
    );
    renderPanel({ duration: 120 });

    await screen.findByText(t.publicBooking.fullDayTitle);
    await waitFor(() => {
      expect(receivedDuration).toBe('120');
    });
  });

  it('leaves the next-day copy out when no day in the window has a free slot', async () => {
    let served = false;
    server.use(
      http.get('*/public/complexes/:slug/next-available', () => {
        served = true;
        return HttpResponse.json({ date: null });
      }),
    );
    renderPanel();

    expect(await screen.findByText(t.publicBooking.fullDayTitle)).toBeInTheDocument();
    // The answer must have arrived before its absence means anything.
    await waitFor(() => {
      expect(served).toBe(true);
    });
    await new Promise((resolve) => {
      setTimeout(resolve, 20);
    });
    expect(screen.queryByText(t.publicBooking.nextAvailableDay, { exact: false })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: t.publicBooking.waitlistSubmit })).toBeInTheDocument();
  });
});

describe('FullDayPanel — WhatsApp to the club', () => {
  it('links to the club with the full day in the prefilled message', async () => {
    renderPanel();

    const link = await screen.findByRole('link', { name: t.publicBooking.whatsAppFullDay });
    const href = new URL(link.getAttribute('href') ?? '');
    expect(href.origin + href.pathname).toBe('https://wa.me/5491155550000');
    expect(href.searchParams.get('text')).toBe(`${t.publicBooking.whatsAppFullDayMessage} 14/10`);
  });

  it('leaves the WhatsApp action out when the club has no usable number', async () => {
    renderPanel({ phone: '' });

    await screen.findByText(t.publicBooking.fullDayTitle);
    expect(screen.queryByRole('link', { name: t.publicBooking.whatsAppFullDay })).not.toBeInTheDocument();
  });
});

describe('FullDayPanel — the waitlist when it works', () => {
  it('sends the trimmed email with the day and confirms it', async () => {
    let receivedBody: unknown;
    server.use(
      http.post('*/public/complexes/:slug/waitlist', async ({ request }) => {
        receivedBody = await request.json();
        return new HttpResponse(null, { status: 204 });
      }),
    );
    renderPanel();

    await screen.findByText(t.publicBooking.fullDayTitle);
    await userEvent.type(emailField(), '  ana@example.com ');
    await submitWaitlist();

    expect(await screen.findByText(confirmation)).toBeInTheDocument();
    expect(receivedBody).toEqual({ date: SELECTED_DAY, email: 'ana@example.com' });
  });

  it('disables the submit button while the request is in flight', async () => {
    let release: () => void = () => undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    waitlistRespondsWith(async () => {
      await gate;
      return new HttpResponse(null, { status: 204 });
    });
    renderPanel();

    await screen.findByText(t.publicBooking.fullDayTitle);
    await userEvent.type(emailField(), 'ana@example.com');
    fireEvent.click(screen.getByRole('button', { name: t.publicBooking.waitlistSubmit }));

    await waitFor(() => {
      expect(screen.getByRole('button', { name: t.publicBooking.waitlistSubmit })).toBeDisabled();
    });
    release();
    expect(await screen.findByText(confirmation)).toBeInTheDocument();
  });
});

describe('FullDayPanel — the waitlist when it fails', () => {
  it('shows a 422 email error under the field', async () => {
    waitlistRespondsWith(() =>
      HttpResponse.json(
        {
          type: 'https://vibe.com.ar/problems/validation',
          title: 'Datos inválidos',
          status: 422,
          errors: [{ field: 'email', message: 'Ingresá un email válido' }],
        },
        { status: 422, headers: { 'Content-Type': 'application/problem+json' } },
      ),
    );
    renderPanel();

    await screen.findByText(t.publicBooking.fullDayTitle);
    await userEvent.type(emailField(), 'nope');
    await submitWaitlist();

    expect(await screen.findByText('Ingresá un email válido')).toBeInTheDocument();
    expect(emailField()).toHaveAttribute('aria-invalid', 'true');
  });

  it('asks to retry in a few minutes on 429', async () => {
    waitlistRespondsWith(() => HttpResponse.json({ title: 'Too many' }, { status: 429 }));
    renderPanel();

    await screen.findByText(t.publicBooking.fullDayTitle);
    await userEvent.type(emailField(), 'ana@example.com');
    await submitWaitlist();

    expect(await screen.findByText(t.publicBooking.waitlistRateLimited)).toBeInTheDocument();
  });

  it('asks to retry on a network failure', async () => {
    waitlistRespondsWith(() => HttpResponse.error());
    renderPanel();

    await screen.findByText(t.publicBooking.fullDayTitle);
    await userEvent.type(emailField(), 'ana@example.com');
    await submitWaitlist();

    expect(await screen.findByText(t.publicBooking.waitlistError)).toBeInTheDocument();
  });
});

describe('FullDayPanel — the saved identity', () => {
  it('prefills the email from the saved identity of this browser', async () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ client_email: 'saved@example.com' }));
    renderPanel();

    await screen.findByText(t.publicBooking.fullDayTitle);
    expect(emailField()).toHaveValue('saved@example.com');
  });
});
