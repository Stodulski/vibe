import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeConsumedHttpError } from '@/test/factories';
import { OpenCashSessionDialog } from './OpenCashSessionDialog';
import { useOpenCashSession } from '../hooks/useOpenCashSession';

vi.mock('../hooks/useOpenCashSession', () => ({ useOpenCashSession: vi.fn() }));

const mutate = vi.fn();

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useOpenCashSession).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
    typeof useOpenCashSession
  >);
});

describe('OpenCashSessionDialog', () => {
  it('rejects submitting with no amount typed', async () => {
    const user = userEvent.setup();
    render(<OpenCashSessionDialog open onClose={vi.fn()} complexId="c1" />);

    await user.click(screen.getByRole('button', { name: 'Abrir caja' }));

    expect(await screen.findByText('Ingresá un monto')).toBeInTheDocument();
    expect(mutate).not.toHaveBeenCalled();
  });

  it('accepts an opening amount of exactly 0 and converts pesos to centavos', async () => {
    const user = userEvent.setup();
    render(<OpenCashSessionDialog open onClose={vi.fn()} complexId="c1" />);

    await user.type(screen.getByLabelText('Monto inicial'), '0');
    await user.click(screen.getByRole('button', { name: 'Abrir caja' }));

    await waitFor(() => {
      expect(mutate).toHaveBeenCalledWith({ opening_cash: 0, note: undefined }, expect.anything());
    });
  });

  it('converts a whole peso amount to integer centavos', async () => {
    const user = userEvent.setup();
    render(<OpenCashSessionDialog open onClose={vi.fn()} complexId="c1" />);

    await user.type(screen.getByLabelText('Monto inicial'), '1500');
    await user.click(screen.getByRole('button', { name: 'Abrir caja' }));

    await waitFor(() => {
      expect(mutate).toHaveBeenCalledWith({ opening_cash: 150000, note: undefined }, expect.anything());
    });
  });

  it('maps a 422 field error from the server onto the amount field', async () => {
    const user = userEvent.setup();
    mutate.mockImplementation((_vars: unknown, options: { onError: (error: unknown) => void }) => {
      void makeConsumedHttpError(422, {
        title: 'Validation Failed',
        errors: [{ field: 'opening_cash', message: 'demasiado grande' }],
      }).then((error) => {
        options.onError(error);
      });
    });
    render(<OpenCashSessionDialog open onClose={vi.fn()} complexId="c1" />);

    await user.type(screen.getByLabelText('Monto inicial'), '5000');
    await user.click(screen.getByRole('button', { name: 'Abrir caja' }));

    expect(await screen.findByText('demasiado grande')).toBeInTheDocument();
  });
});

describe('OpenCashSessionDialog — decimal input never silently corrupts the amount', () => {
  it('strips a typed "." instead of treating it as a decimal point — whole pesos only', async () => {
    const user = userEvent.setup();
    render(<OpenCashSessionDialog open onClose={vi.fn()} complexId="c1" />);

    // `MoneyPesosField` is a `type="text"` money input now (see its own doc
    // comment) — there is no native number-input step to mismatch any more.
    // A typed "." is just a stray character on this field (es-AR's own
    // decimal separator is ",", not "."), so it is stripped like any other
    // non-digit: "1500.5" becomes the whole-pesos amount 15005, not a
    // rejected 1500.5.
    await user.type(screen.getByLabelText('Monto inicial'), '1500.5');
    expect(screen.getByLabelText('Monto inicial')).toHaveValue('15.005');

    await user.click(screen.getByRole('button', { name: 'Abrir caja' }));

    await waitFor(() => {
      expect(mutate).toHaveBeenCalledWith({ opening_cash: 1500500, note: undefined }, expect.anything());
    });
  });

  it('rejects a decimal amount typed with a comma with the app\'s own "whole pesos" error, and never submits', async () => {
    const user = userEvent.setup();
    render(<OpenCashSessionDialog open onClose={vi.fn()} complexId="c1" />);

    // Regression coverage: typing "1500,50" one keystroke at a time used to
    // reformat the comma away the instant it landed, so the following "5"
    // and "0" silently read as two more thousands digits of the integer part
    // — "1500,50" ended up recording 150050 pesos, a 100x amount, with no
    // validation error anywhere (see `useMoneyInput`'s own doc comment). The
    // field now shows the amount back exactly as typed and reports the real
    // fraction (1500.5), so `sessionCashSchema`'s `.int()` rejects it visibly.
    await user.type(screen.getByLabelText('Monto inicial'), '1500,50');
    expect(screen.getByLabelText('Monto inicial')).toHaveValue('1.500,50');

    await user.click(screen.getByRole('button', { name: 'Abrir caja' }));

    expect(await screen.findByText('El monto debe ser en pesos enteros, sin centavos')).toBeInTheDocument();
    expect(mutate).not.toHaveBeenCalled();
  });
});
