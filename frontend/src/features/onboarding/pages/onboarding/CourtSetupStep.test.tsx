import { describe, it, expect, vi } from 'vitest';
import userEvent from '@testing-library/user-event';
import { render, screen } from '@testing-library/react';
import { CourtSetupStep } from './CourtSetupStep';

vi.mock('@/shared/components/ui/button', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Button: passthrough('button') };
});
vi.mock('@/shared/components/ui/input', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Input: passthrough('input') };
});
vi.mock('@/shared/components/ui/label', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Label: passthrough('label') };
});
vi.mock('@/shared/components/ui/select', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return {
    Select: passthrough('div'),
    SelectContent: passthrough('div'),
    SelectItem: passthrough('div'),
    SelectTrigger: passthrough('div'),
    SelectValue: () => null,
  };
});
vi.mock('@/features/courts/components/PriceConfig', () => ({ PriceConfig: () => null }));
vi.mock('@/features/courts/schemas/courts.schema', () => ({
  createCourtSchema: { parse: vi.fn() },
}));
vi.mock('@hookform/resolvers/zod', () => ({ zodResolver: () => vi.fn() }));
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }));

const baseProps = {
  complexId: 'c1',
  courts: [],
  hasCourts: false,
  createCourt: { mutate: vi.fn(), isPending: false },
  deleteCourt: { mutate: vi.fn(), isPending: false },
  onBack: vi.fn(),
  onNext: vi.fn(),
};

describe('CourtSetupStep', () => {
  it('says why the step exists, without repeating its name from the indicator', () => {
    render(<CourtSetupStep {...baseProps} />);
    expect(screen.getByText(/nada que reservar/i)).toBeInTheDocument();
  });

  it('renders court name input', () => {
    render(<CourtSetupStep {...baseProps} />);
    expect(screen.getByPlaceholderText('Ej: Cancha 1')).toBeInTheDocument();
  });

  it('labels the form submit as adding the court, not as opening a new one', () => {
    render(<CourtSetupStep {...baseProps} />);
    expect(screen.getByRole('button', { name: 'Agregar cancha' })).toHaveAttribute('type', 'submit');
    expect(screen.queryByRole('button', { name: 'Nueva cancha' })).not.toBeInTheDocument();
  });

  it('renders back button', () => {
    render(<CourtSetupStep {...baseProps} />);
    expect(screen.getByText('Volver')).toBeInTheDocument();
  });

  // The gate is still there — without a court there is nothing to book — but a
  // real `disabled` attribute takes the button out of the tab order, so a
  // keyboard or screen-reader user reached the end of this screen and the
  // control simply was not there. It stays reachable and says why on press.
  it('stays reachable without courts and explains itself on press', async () => {
    const user = userEvent.setup();
    render(<CourtSetupStep {...baseProps} />);

    const nextBtn = screen.getByRole('button', { name: /siguiente/i });
    expect(nextBtn).toBeEnabled();

    await user.click(nextBtn);

    expect(await screen.findByRole('alert')).toHaveTextContent(/al menos una cancha/i);
    expect(baseProps.onNext).not.toHaveBeenCalled();
  });

  it('advances once a court exists', async () => {
    const user = userEvent.setup();
    render(
      <CourtSetupStep
        {...baseProps}
        hasCourts={true}
        courts={[{ id: '1', name: 'C1', sport: 'padel', court_type: 'outdoor' }]}
      />,
    );

    await user.click(screen.getByRole('button', { name: /siguiente/i }));

    expect(baseProps.onNext).toHaveBeenCalled();
  });

  it('shows existing courts', () => {
    render(
      <CourtSetupStep
        {...baseProps}
        hasCourts={true}
        courts={[{ id: '1', name: 'Cancha Test', sport: 'padel', court_type: 'outdoor' }]}
      />,
    );
    expect(screen.getByText('Cancha Test')).toBeInTheDocument();
  });

  it("names each court's icon-only delete button by the court, for screen readers", () => {
    render(
      <CourtSetupStep
        {...baseProps}
        hasCourts={true}
        courts={[{ id: '1', name: 'Cancha Test', sport: 'padel', court_type: 'outdoor' }]}
      />,
    );
    expect(screen.getByRole('button', { name: 'Eliminar: Cancha Test' })).toBeInTheDocument();
  });
});
