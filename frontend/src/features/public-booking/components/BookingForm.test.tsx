import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { hasUnsavedWork } from '@/shared/lib/unsavedWork';
import { STORAGE_KEY, getSavedFormData, hasCompleteSavedData } from './booking-form/savedFormData';
import { BookingForm, type BookingSlotInfo } from './BookingForm';

const SAVED_IDENTITY = {
  client_first_name: 'Juan',
  client_last_name: 'Perez',
  client_phone: '1123456789',
  client_email: 'juan@example.com',
};

// Clear localStorage before each test
beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
});

const mockSlotInfo: BookingSlotInfo = {
  complexId: 'c1',
  complexName: 'Padel Club Norte',
  complexPhone: '1155550000',
  courtId: 'ct1',
  courtName: 'Cancha 1',
  date: '2026-03-20',
  startTime: '10:00',
  endTime: '11:30',
  durationMinutes: 90,
  price: 1500000,
  depositPercentage: 30,
  cancellationHours: 24,
};

const defaultProps = {
  slotInfo: mockSlotInfo,
  onSubmit: vi.fn(),
  isLoading: false,
};

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.clear();
  sessionStorage.clear();
});

describe('BookingForm summary content', () => {
  it('renders court name in summary', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getByText('Cancha 1')).toBeInTheDocument();
  });

  it('renders time range in summary', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getByText(/10:00/)).toBeInTheDocument();
    expect(screen.getByText(/11:30/)).toBeInTheDocument();
  });

  it('renders deposit amount in summary', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getByText(/30%/)).toBeInTheDocument();
  });

  it('renders service fee in summary', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getAllByText(/cargo de servicio/i).length).toBeGreaterThan(0);
  });

  it('renders total online amount in the summary', () => {
    render(<BookingForm {...defaultProps} />);
    const summary = screen.getByRole('region', { name: /resumen de la reserva/i });
    expect(within(summary).getByText(/pag.s ahora/i)).toBeInTheDocument();
  });

  // U-05: the pay button now restates the total beside itself (moved out of
  // the button's own label, which used to force it onto two lines), so the
  // amount is confirmed once in the summary above and once right by the
  // action — not the same element twice.
  it('restates the total online amount beside the pay button', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getAllByText(/pag.s ahora/i)).toHaveLength(2);
  });

  it('renders the service-fee helper line under the fee row instead of the old footnote', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getByText(/no se descuenta del precio de la cancha/i)).toBeInTheDocument();
  });
});

describe('BookingForm form fields', () => {
  it('renders first name input', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getByLabelText(/nombre/i)).toBeInTheDocument();
  });

  it('renders last name input', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getByLabelText(/apellido/i)).toBeInTheDocument();
  });

  it('renders phone input', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getByLabelText(/tel.fono/i)).toBeInTheDocument();
  });

  it('renders email input', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getByLabelText(/email/i)).toBeInTheDocument();
  });

  it('shows the fixed +54 prefix and no selector', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getByText('+54')).toBeInTheDocument();
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
  });

  it('renders notes toggle button', () => {
    render(<BookingForm {...defaultProps} />);
    expect(screen.getByText(/notas/i)).toBeInTheDocument();
  });
});

describe('BookingForm saves client data without writing on every render', () => {
  it('does not write to localStorage merely from mounting, before any field changes', () => {
    const setItemSpy = vi.spyOn(Storage.prototype, 'setItem');
    render(<BookingForm {...defaultProps} />);
    expect(setItemSpy).not.toHaveBeenCalledWith(STORAGE_KEY, expect.anything());
  });

  it('does not write again on a re-render with no field change', () => {
    const { rerender } = render(<BookingForm {...defaultProps} />);
    const setItemSpy = vi.spyOn(Storage.prototype, 'setItem');
    rerender(<BookingForm {...defaultProps} />);
    expect(setItemSpy).not.toHaveBeenCalledWith(STORAGE_KEY, expect.anything());
  });

  it('writes the typed value to localStorage once a field actually changes', async () => {
    const user = userEvent.setup();
    render(<BookingForm {...defaultProps} />);
    await user.type(screen.getByLabelText(/nombre/i), 'Juan');

    const stored = localStorage.getItem(STORAGE_KEY);
    expect(stored).not.toBeNull();
    expect(JSON.parse(stored ?? '{}')).toMatchObject({ client_first_name: 'Juan' });
  });
});

describe('BookingForm submit button', () => {
  it('renders submit button with price', () => {
    render(<BookingForm {...defaultProps} />);
    const button = screen.getByRole('button', { name: /pagar|reservar/i });
    expect(button).toBeInTheDocument();
  });

  it('disables submit button when loading', () => {
    const { container } = render(<BookingForm {...defaultProps} isLoading />);
    const button = container.querySelector('button[type="submit"]');
    if (!button) throw new Error('submit button not found');
    expect(button).toBeDisabled();
  });

  // The button shows only a spinner icon while loading — aria-busy plus the
  // visually-hidden text is what tells a screen reader something is
  // happening, since there is no visible label to announce.
  it('marks the submit button aria-busy and names what it is doing, while loading', () => {
    const { container } = render(<BookingForm {...defaultProps} isLoading />);
    const button = container.querySelector('button[type="submit"]');
    if (!button) throw new Error('submit button not found');
    expect(button).toHaveAttribute('aria-busy', 'true');
    expect(button).toHaveTextContent(/generando el link de pago/i);
  });
});

// M11: this form had no test for its own validation — submitting empty
// required fields, or an invalid phone number — even though it is the last
// step before a real payment.
describe('BookingForm validation', () => {
  it('shows required-field errors and does not call onSubmit when every field is empty', async () => {
    const user = userEvent.setup();
    render(<BookingForm {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /pagar/i }));

    expect(await screen.findByText(/el nombre es requerido/i)).toBeInTheDocument();
    expect(screen.getByText(/el apellido es requerido/i)).toBeInTheDocument();
    expect(screen.getByText(/tel.fono inv.lido/i)).toBeInTheDocument();
    expect(defaultProps.onSubmit).not.toHaveBeenCalled();
  });

  it('marks an empty required field aria-invalid, tied to its error via aria-describedby', async () => {
    const user = userEvent.setup();
    render(<BookingForm {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /pagar/i }));
    await screen.findByText(/el nombre es requerido/i);

    const firstName = screen.getByLabelText(/nombre/i);
    expect(firstName).toHaveAttribute('aria-invalid', 'true');
    expect(firstName).toHaveAttribute('aria-describedby', 'client_first_name-error');
  });

  it('every validation error announces through role="alert"', async () => {
    const user = userEvent.setup();
    render(<BookingForm {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /pagar/i }));
    const alerts = await screen.findAllByRole('alert');
    expect(alerts.length).toBeGreaterThanOrEqual(3);
  });

  it('rejects an incomplete phone number and does not call onSubmit', async () => {
    const user = userEvent.setup();
    render(<BookingForm {...defaultProps} />);

    await user.type(screen.getByLabelText(/nombre/i), 'Juan');
    await user.type(screen.getByLabelText(/apellido/i), 'Pérez');
    await user.type(screen.getByLabelText(/tel.fono/i), '123');
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    expect(await screen.findByText(/tel.fono inv.lido/i)).toBeInTheDocument();
    expect(defaultProps.onSubmit).not.toHaveBeenCalled();
  });

  it('calls onSubmit once every required field is valid', async () => {
    const user = userEvent.setup();
    render(<BookingForm {...defaultProps} />);

    await user.type(screen.getByLabelText(/nombre/i), 'Juan');
    await user.type(screen.getByLabelText(/apellido/i), 'Pérez');
    await user.type(screen.getByLabelText(/tel.fono/i), '1123456789');
    await user.click(screen.getByRole('button', { name: /pagar/i }));

    expect(defaultProps.onSubmit).toHaveBeenCalledTimes(1);
    expect(screen.queryByText(/el nombre es requerido/i)).not.toBeInTheDocument();
  });
});

// SEC-05: quick-book's "No, soy otra persona" must forget the saved identity
// (not just hide it) and hand the visitor a genuinely empty form — the two
// views share one react-hook-form instance, whose `defaultValues` are
// captured once at mount, from whatever was saved.
describe('BookingForm quick-book "No, soy otra persona"', () => {
  it('enters quick-book with a complete saved identity', () => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(SAVED_IDENTITY));
    render(<BookingForm {...defaultProps} />);

    expect(screen.getByText(/juan perez/i)).toBeInTheDocument();
  });

  it('clears storage and shows an empty full form after "No, soy otra persona"', async () => {
    const user = userEvent.setup();
    localStorage.setItem(STORAGE_KEY, JSON.stringify(SAVED_IDENTITY));
    render(<BookingForm {...defaultProps} />);

    await user.click(screen.getByRole('button', { name: /no, soy otra persona/i }));

    // The autosave hook (`useSaveFormData`) persists the now-empty fields
    // right after `reset()`, so the key is not simply gone — what matters is
    // that Juan's identity is: a fresh visit reading this back would find it
    // incomplete and never quick-book with it again.
    expect(hasCompleteSavedData(getSavedFormData())).toBe(false);
    expect(screen.getByLabelText(/nombre/i)).toHaveValue('');
    expect(screen.getByLabelText(/apellido/i)).toHaveValue('');
    expect(screen.getByLabelText(/email/i)).toHaveValue('');
  });
});

// PWA-09: the PWA applies a waiting build when the tab goes to the background.
// A client typing their details, switching to WhatsApp to check a phone number
// and coming back must find the form as they left it.
describe('BookingForm unsaved work', () => {
  it('reports no unsaved work from merely rendering the empty form', () => {
    render(<BookingForm {...defaultProps} />);

    expect(hasUnsavedWork()).toBe(false);
  });

  it('marks the app busy once a field is typed into', async () => {
    const user = userEvent.setup();
    render(<BookingForm {...defaultProps} />);

    await user.type(screen.getByLabelText(/nombre/i), 'Juan');

    expect(hasUnsavedWork()).toBe(true);
  });

  it('clears the registration when the form unmounts', async () => {
    const user = userEvent.setup();
    const { unmount } = render(<BookingForm {...defaultProps} />);

    await user.type(screen.getByLabelText(/nombre/i), 'Juan');
    expect(hasUnsavedWork()).toBe(true);

    unmount();

    expect(hasUnsavedWork()).toBe(false);
  });
});
