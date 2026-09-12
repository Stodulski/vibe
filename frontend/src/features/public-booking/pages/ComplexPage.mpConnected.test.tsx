import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import type { AvailabilityData, PublicComplexResponse } from '@/shared/types/api.types';
import type { BookingSlotInfo } from '@/features/public-booking';

// ─── Regression test for the server contract migration ───
//
// `GET /api/v1/public/complexes/:slug` used to serialise `mp_user_id`. The
// server dropped it (it leaked the MercadoPago collector id the payment path
// checks incoming payments against) and replaced it with `payments_enabled`.
// `mp_user_id` is optional on the old `Complex` type, so a client still
// reading it compiles clean and always sees `undefined` — a silent,
// permanent "not connected" for every venue. This test renders the real
// booking chain so the assertion is on the *observable* consequence, not on
// the intermediate boolean.
//
// What that consequence looks like changed. A venue without online payments
// used to render its whole availability grid with every slot disabled, under
// a warning banner. Now it renders no grid at all — just its phone number —
// so "cannot be booked online" is asserted by the absence of slots and the
// presence of the number, rather than by hundreds of disabled buttons.

const mockNavigate = vi.fn();

vi.mock('react-router-dom', async () => {
  const actual = await vi.importActual('react-router-dom');
  return { ...actual, useNavigate: () => mockNavigate };
});

vi.mock('@/shared/hooks/usePageTitle', () => ({
  usePageTitle: vi.fn(),
  useOGTags: vi.fn(),
  useCanonical: vi.fn(),
  useStructuredData: vi.fn(),
}));

const mockUseComplexBySlug =
  vi.fn<() => { data: PublicComplexResponse | undefined; isLoading: boolean; error: unknown }>();
vi.mock('@/features/public-booking/hooks/useComplexBySlug', () => ({
  useComplexBySlug: () => mockUseComplexBySlug(),
}));

const mockUseAvailability = vi.fn<() => { data: AvailabilityData | undefined; isLoading: boolean }>();
vi.mock('@/features/public-booking/hooks/useAvailability', () => ({
  useAvailability: () => mockUseAvailability(),
}));

// Irrelevant to this test — kept as inert stubs, same as ComplexPage.test.tsx.
vi.mock('@/features/public-booking/components/ComplexHeader', () => ({
  ComplexHeader: () => <div data-testid="complex-header">Header</div>,
}));
vi.mock('@/features/public-booking/components/DateSelector', () => ({
  DateSelector: () => <div data-testid="date-selector">DateSelector</div>,
}));

// CourtSelector, PhoneBookingPanel, and everything below them are the REAL
// components — the whole point is to observe the slots, the phone number and
// the Continue button they actually render, not a stand-in.

function buildComplexData(paymentsEnabled: boolean): PublicComplexResponse {
  return {
    complex: {
      id: 'c1',
      name: 'Club Test',
      slug: 'test-club',
      amenities: [],
      address: 'Calle Falsa 123',
      city: 'CABA',
      province: 'CABA',
      country_code: 'AR',
      currency: 'ARS',
      phone: PHONE,
      deposit_percentage: 30,
      cancellation_hours: 24,
      payments_enabled: paymentsEnabled,
    },
    courts: [],
    schedules: [],
  };
}

function buildAvailability(): AvailabilityData {
  return {
    date: '2026-08-24',
    day: 'monday',
    is_open: true,
    courts: [
      {
        court_id: 'ct1',
        court_name: 'Cancha 1',
        sport: 'padel',
        court_type: 'indoor',
        duration_minutes: 60,
        slots: [
          {
            start_time: '10:00',
            end_time: '11:00',
            start_min: 600,
            duration_minutes: 60,
            price: 5000,
            available: true,
          },
        ],
      },
    ],
  };
}

const PHONE = '+541100000000';

async function renderComplexPage() {
  const Page = (await import('./ComplexPage')).default;
  return render(
    <MemoryRouter initialEntries={['/test-club']}>
      <Routes>
        <Route path="/:slug" element={<Page />} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('ComplexPage — mpConnected derived from payments_enabled', () => {
  beforeEach(() => {
    mockNavigate.mockClear();
    mockUseAvailability.mockReturnValue({ data: buildAvailability(), isLoading: false });
  });

  it('payments_enabled: true — slots are bookable and the Continue path is reachable', async () => {
    mockUseComplexBySlug.mockReturnValue({
      data: buildComplexData(true),
      isLoading: false,
      error: null,
    });
    await renderComplexPage();

    expect(screen.queryByRole('link', { name: /llamar al complejo/i })).not.toBeInTheDocument();

    // The hours are behind the duration question now: the flow asks what
    // changes the inventory before showing what is in it. This fixture has a
    // single sport, so that step is skipped and the duration is the only one.
    fireEvent.click(screen.getByRole('button', { name: '90 min' }));

    // The hour button's accessible name is the time alone now; the price
    // moved to the court cards.
    const slotButton = screen.getByRole('button', { name: /^10:00\b/ });
    expect(slotButton).not.toBeDisabled();
    fireEvent.click(slotButton);

    const continueButton = await screen.findByRole('button', { name: /continuar/i });
    fireEvent.click(continueButton);

    expect(mockNavigate).toHaveBeenCalledWith(
      '/test-club/book/confirm',
      expect.objectContaining({
        state: expect.objectContaining({ complexId: 'c1' }) as unknown as BookingSlotInfo,
      }),
    );
  });

  it('asks the duration before the hours, and only once', async () => {
    // The duration used to be drawn inside each court card — twelve courts,
    // twelve copies of one control — then became a filter above the grid, and
    // is now the question that precedes it. What has to stay true through all
    // three shapes is that it is asked in exactly one place, and that the
    // hours are not shown until it has an answer: the server builds the grid
    // of start times from the duration, so hours chosen before it are hours
    // that might not exist.
    mockUseComplexBySlug.mockReturnValue({
      data: buildComplexData(true),
      isLoading: false,
      error: null,
    });
    await renderComplexPage();

    expect(screen.getByRole('heading', { name: /cuánto tiempo/i })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /^10:00\b/ })).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: '90 min' }));

    expect(screen.queryByRole('heading', { name: /cuánto tiempo/i })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^10:00\b/ })).toBeInTheDocument();
    // And it stays reachable, as the breadcrumb that shows what was answered.
    expect(screen.getAllByRole('button', { name: /duración.*90 min/i })).toHaveLength(1);
  });

  it('payments_enabled: false — the phone replaces the grid and the Continue path is unreachable', async () => {
    mockUseComplexBySlug.mockReturnValue({
      data: buildComplexData(false),
      isLoading: false,
      error: null,
    });
    await renderComplexPage();

    // The number opens a WhatsApp chat with the club, not merely printed somewhere.
    const call = screen.getByRole('link', { name: /escribir al complejo por whatsapp/i });
    expect(call.getAttribute('href')).toMatch(new RegExp(`^https://wa\\.me/${PHONE.replace(/\\D/g, '')}\\?text=`));
    expect(call).toHaveAttribute('target', '_blank');
    expect(call).toHaveTextContent(PHONE);

    // No slot survives — not disabled, absent. A disabled button is still an
    // invitation the venue cannot honour.
    expect(screen.queryByRole('button', { name: /^10:00\b/ })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /continuar/i })).not.toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
