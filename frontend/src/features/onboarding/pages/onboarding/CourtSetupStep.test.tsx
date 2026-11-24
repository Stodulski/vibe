import { describe, it, expect, vi, beforeEach } from 'vitest';
import userEvent from '@testing-library/user-event';
import { act, render, screen } from '@testing-library/react';
import { makeCourt, makePrice } from '@/test/factories';
import type { CourtPrice, CourtWithPrices } from '@/shared/types/api.types';
import { CourtSetupStep } from './CourtSetupStep';

vi.mock('@/shared/components/ui/button', async () => {
  const { passthrough } = await import('@/test/ui-mocks');
  return { Button: passthrough('button') };
});

// The court dialog and the price dialog are exercised by their own tests. Here
// they stand in as markers, so these cases can say when the step opens each one
// without dragging Radix, react-hook-form and two mutations into every render.
const courtFormProps = vi.fn();
vi.mock('@/features/courts/components/CourtForm', () => ({
  CourtForm: (props: { open: boolean; onCreated?: (court: unknown) => void }) => {
    courtFormProps(props);
    return props.open ? <div data-testid="court-form" /> : null;
  },
}));
vi.mock('@/features/courts/components/PriceConfig', () => ({
  PriceConfig: () => <div data-testid="price-config" />,
}));

function court({ name = 'C1', prices = [] }: { name?: string; prices?: CourtPrice[] } = {}): CourtWithPrices {
  return { ...makeCourt({ id: '1', name }), prices };
}

const baseProps = {
  complexId: 'c1',
  courts: [],
  hasCourts: false,
  deleteCourt: { mutate: vi.fn(), isPending: false },
  onBack: vi.fn(),
  onNext: vi.fn(),
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('CourtSetupStep', () => {
  // The form is behind a button rather than filling the step: the owner sees
  // what they have and one thing to do, instead of a form for a court they may
  // already have added.
  it('offers one button and no inline form until it is pressed', () => {
    render(<CourtSetupStep {...baseProps} />);

    expect(screen.getByRole('button', { name: 'Agregá tu primera cancha' })).toBeInTheDocument();
    expect(screen.queryByTestId('court-form')).not.toBeInTheDocument();
  });

  it('opens the court dialog on press', async () => {
    const user = userEvent.setup();
    render(<CourtSetupStep {...baseProps} />);

    await user.click(screen.getByRole('button', { name: 'Agregá tu primera cancha' }));

    expect(screen.getByTestId('court-form')).toBeInTheDocument();
  });

  it('names the button for what it adds once a court exists', () => {
    render(<CourtSetupStep {...baseProps} hasCourts={true} courts={[court({ name: 'C1' })]} />);

    expect(screen.getByRole('button', { name: 'Agregar otra cancha' })).toBeInTheDocument();
  });

  // A court with no price is not bookable, so the price dialog follows the
  // court dialog rather than waiting to be found on the courts page.
  it('chains into pricing the court it just created', () => {
    render(<CourtSetupStep {...baseProps} />);

    expect(screen.queryByTestId('price-config')).not.toBeInTheDocument();

    const { onCreated } = courtFormProps.mock.calls.at(-1)?.[0] as { onCreated: (c: unknown) => void };
    act(() => {
      onCreated({ id: '1', name: 'C1', sport: 'padel', court_type: 'outdoor', prices: [] });
    });

    expect(screen.getByTestId('price-config')).toBeInTheDocument();
  });
});

describe('CourtSetupStep navigation', () => {
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
    render(<CourtSetupStep {...baseProps} hasCourts={true} courts={[court({ name: 'C1' })]} />);

    await user.click(screen.getByRole('button', { name: /siguiente/i }));

    expect(baseProps.onNext).toHaveBeenCalled();
  });

  it('shows existing courts', () => {
    render(<CourtSetupStep {...baseProps} hasCourts={true} courts={[court({ name: 'Cancha Test' })]} />);
    expect(screen.getByText('Cancha Test')).toBeInTheDocument();
  });

  it("names each court's icon-only delete button by the court, for screen readers", () => {
    render(<CourtSetupStep {...baseProps} hasCourts={true} courts={[court({ name: 'Cancha Test' })]} />);
    expect(screen.getByRole('button', { name: 'Eliminar: Cancha Test' })).toBeInTheDocument();
  });
});

describe('CourtSetupStep pricing', () => {
  // The price dialog opens once, right after creation, and can be dismissed.
  // The court then has to say it is not bookable yet and offer the way back.
  it('shows a court without prices as unpriced and reopens its price dialog', async () => {
    const user = userEvent.setup();
    render(<CourtSetupStep {...baseProps} hasCourts={true} courts={[court({ name: 'Cancha 1' })]} />);

    expect(screen.getByText('Sin precios configurados')).toBeInTheDocument();
    expect(screen.queryByTestId('price-config')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Cargar precios: Cancha 1' }));

    expect(screen.getByTestId('price-config')).toBeInTheDocument();
  });

  it('does not flag a court that has prices', () => {
    render(
      <CourtSetupStep
        {...baseProps}
        hasCourts={true}
        courts={[court({ name: 'Cancha 1', prices: [makePrice({ price: 10000 })] })]}
      />,
    );

    expect(screen.queryByText('Sin precios configurados')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Cargar precios: Cancha 1' })).not.toBeInTheDocument();
  });
});

describe('CourtSetupStep court list', () => {
  // The courts page card says what each court is made of and whether it has
  // lights; the onboarding list used to stop at sport and roof.
  it('shows the surface and lighting of each court, as the courts page does', () => {
    render(
      <CourtSetupStep
        {...baseProps}
        hasCourts={true}
        courts={[
          {
            ...makeCourt({
              id: '1',
              name: 'Cancha 1',
              court_type: 'indoor',
              surface: 'synthetic_grass',
              has_lighting: true,
            }),
            prices: [],
          },
          {
            ...makeCourt({ id: '2', name: 'Cancha 2', court_type: 'outdoor', surface: null, has_lighting: false }),
            prices: [],
          },
        ]}
      />,
    );
    expect(screen.getByText('Césped sintético · Con iluminación')).toBeInTheDocument();
    expect(screen.getAllByText(/Césped|iluminación/)).toHaveLength(1);
  });
});
