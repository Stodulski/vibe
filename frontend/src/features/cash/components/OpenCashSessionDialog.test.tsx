import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeConsumedHttpError } from '@/test/factories';
import { OpenCashSessionDialog } from './OpenCashSessionDialog';
import { useOpenCashSession } from '../hooks/useOpenCashSession';

vi.mock('../hooks/useOpenCashSession', () => ({ useOpenCashSession: vi.fn() }));

describe('OpenCashSessionDialog', () => {
  const mutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useOpenCashSession).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
      typeof useOpenCashSession
    >);
  });

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

  it('rejects a decimal amount with the app\'s own "whole pesos" error, not a native step-mismatch block', async () => {
    const user = userEvent.setup();
    render(<OpenCashSessionDialog open onClose={vi.fn()} complexId="c1" />);

    // Without `noValidate` on the `<form>`, a number input's default step of
    // 1 makes the browser refuse to submit at all here — no validation error
    // rendered, no `mutate` call, just nothing (see MoneyPesosField's doc
    // comment). `noValidate` lets react-hook-form/Zod run instead.
    await user.type(screen.getByLabelText('Monto inicial'), '1500.5');
    await user.click(screen.getByRole('button', { name: 'Abrir caja' }));

    expect(await screen.findByText('El monto debe ser en pesos enteros, sin centavos')).toBeInTheDocument();
    expect(mutate).not.toHaveBeenCalled();
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
