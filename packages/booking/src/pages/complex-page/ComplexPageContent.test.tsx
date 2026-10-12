import type { ComponentProps } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { screen } from '@testing-library/react';
import { renderBooking } from '../../test/booking';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicComplex } from '../../api/types';
import { ComplexPageContent } from './ComplexPageContent';

const WIDE_QUERY = '(min-width: 1024px)';

const complex: PublicComplex = {
  id: 'c1',
  name: 'Club Padel Norte',
  slug: 'club-padel-norte',
  amenities: ['parking', 'wifi'],
  address: 'Av. Libertador 1234',
  city: 'Buenos Aires',
  province: 'CABA',
  country_code: 'AR',
  currency: 'ARS',
  phone: '+5491155550000',
  deposit_percentage: 30,
  cancellation_hours: 24,
  payments_enabled: true,
};

const baseProps: ComponentProps<typeof ComplexPageContent> = {
  complex,
  schedules: [],
  courts: [],
  mpConnected: true,
  selectedDate: new Date(2026, 2, 20),
  onDateSelect: vi.fn(),
  availableSports: [],
  sportFilter: null,
  onSportFilterChange: vi.fn(),
  availLoading: false,
  availError: false,
  onAvailRetry: vi.fn(),
  availStale: false,
  availability: undefined,
  dateStr: '2026-03-20',
  filteredCourts: [],
  selectedSlot: null,
  onSelectSlot: vi.fn(),
  onContinue: vi.fn(),
  duration: 90,
  selectedDuration: null,
  onDurationChange: vi.fn(),
  pendingStartTime: null,
  onPendingStartTimeChange: vi.fn(),
  answeredFromUrl: { sport: false, duration: false },
};

function viewport(wide: boolean) {
  vi.spyOn(window, 'matchMedia').mockImplementation(query => ({
    matches: query === WIDE_QUERY && wide,
    media: query,
    onchange: null,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    addListener: () => undefined,
    removeListener: () => undefined,
    dispatchEvent: () => false,
  }));
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe('ComplexPageContent placement of the services', () => {
  it('puts the services after the date picker on a phone, and only once', () => {
    viewport(false);
    renderBooking(<ComplexPageContent {...baseProps} />);

    const services = screen.getByRole('heading', { level: 2, name: BOOKING_MESSAGES.complex.amenitiesSection });
    const dates = screen.getByRole('radiogroup', { name: BOOKING_MESSAGES.publicBooking.dateStripLabel });
    expect(dates.compareDocumentPosition(services) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('keeps the services inside the club card on a wide screen, and only once', () => {
    viewport(true);
    renderBooking(<ComplexPageContent {...baseProps} />);

    const services = screen.getByRole('heading', { level: 2, name: BOOKING_MESSAGES.complex.amenitiesSection });
    const dates = screen.getByRole('radiogroup', { name: BOOKING_MESSAGES.publicBooking.dateStripLabel });
    expect(dates.compareDocumentPosition(services) & Node.DOCUMENT_POSITION_PRECEDING).toBeTruthy();
  });
});
