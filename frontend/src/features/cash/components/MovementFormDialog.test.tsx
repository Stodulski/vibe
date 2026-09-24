import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeConsumedHttpError } from '@/test/factories';
import { MovementFormDialog } from './MovementFormDialog';
import { useCreateCashMovement } from '../hooks/useCreateCashMovement';

vi.mock('../hooks/useCreateCashMovement', () => ({ useCreateCashMovement: vi.fn() }));

const INCOME_CATEGORY_LABELS = [
  'Clases',
  'Torneos',
  'Eventos',
  'Cuotas y abonos',
  'Publicidad y sponsors',
  'Aporte / cambio',
  'Otros ingresos',
];
const EXPENSE_CATEGORY_LABELS = [
  'Insumos',
  'Sueldos',
  'Servicios',
  'Mantenimiento',
  'Limpieza',
  'Retiro',
  'Alquiler del local',
  'Impuestos',
  'Honorarios',
  'Marketing',
  'Comisiones bancarias',
  'Otros egresos',
];

// "Categoría" is the first combobox; "Método" is the second (see
// `ConfirmPaymentModal.test.tsx` for the same positional pattern — a Radix
// `Select` root is treated as a wrapper by `FormField`'s auto-wiring, so its
// accessible name is not reliably queryable by the label text). Shared by
// both category-list assertions below so neither `it` block pushes the
// `describe` over the line-count limit.
async function expectCategoryOptions(kind: 'income' | 'expense', labels: string[]) {
  const user = userEvent.setup();
  render(<MovementFormDialog open onClose={vi.fn()} complexId="c1" sessionId="s1" kind={kind} />);
  const [category] = screen.getAllByRole('combobox');
  if (!category) throw new Error('category combobox not found');
  expect(category).toHaveTextContent(labels[0] ?? '');

  await user.click(category);
  const options = screen.getAllByRole('option').map((o) => o.textContent);
  expect(options).toEqual(labels);
}

describe('MovementFormDialog', () => {
  const mutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useCreateCashMovement).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
      typeof useCreateCashMovement
    >);
  });

  it('defaults an income movement to Clases and lists the seven income categories, "Otros ingresos" last', async () => {
    await expectCategoryOptions('income', INCOME_CATEGORY_LABELS);
  });

  it('defaults an expense movement to Insumos and lists the twelve expense categories, "Otros egresos" last', async () => {
    await expectCategoryOptions('expense', EXPENSE_CATEGORY_LABELS);
  });

  it('rejects submitting with no amount typed', async () => {
    const user = userEvent.setup();
    render(<MovementFormDialog open onClose={vi.fn()} complexId="c1" sessionId="s1" kind="expense" />);

    await user.click(screen.getByRole('button', { name: 'Egreso' }));

    expect(await screen.findByText('Ingresá un monto')).toBeInTheDocument();
    expect(mutate).not.toHaveBeenCalled();
  });

  it('converts the typed pesos amount to centavos on submit', async () => {
    const user = userEvent.setup();
    render(<MovementFormDialog open onClose={vi.fn()} complexId="c1" sessionId="s1" kind="expense" />);

    await user.type(screen.getByLabelText('Monto'), '1500');
    await user.click(screen.getByRole('button', { name: 'Egreso' }));

    await waitFor(() => {
      expect(mutate).toHaveBeenCalledWith(
        { kind: 'expense', category: 'supplies', method: 'cash', amount: 150000, note: undefined },
        expect.anything(),
      );
    });
  });

  it('maps a 422 field error from the server onto the amount field', async () => {
    const user = userEvent.setup();
    mutate.mockImplementation((_vars: unknown, options: { onError: (error: unknown) => void }) => {
      void makeConsumedHttpError(422, {
        title: 'Validation Failed',
        errors: [{ field: 'amount', message: 'demasiado grande' }],
      }).then((error) => {
        options.onError(error);
      });
    });
    render(<MovementFormDialog open onClose={vi.fn()} complexId="c1" sessionId="s1" kind="expense" />);

    await user.type(screen.getByLabelText('Monto'), '5000');
    await user.click(screen.getByRole('button', { name: 'Egreso' }));

    expect(await screen.findByText('demasiado grande')).toBeInTheDocument();
  });
});
