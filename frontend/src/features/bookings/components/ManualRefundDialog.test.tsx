import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ManualRefundDialog } from './ManualRefundDialog';

describe('ManualRefundDialog', () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    onConfirm: vi.fn(),
    isLoading: false,
    amount: 15000,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the outstanding amount', () => {
    render(<ManualRefundDialog {...defaultProps} />);
    expect(screen.getByText(/\$150\b/)).toBeInTheDocument();
  });

  it('calls onConfirm when confirm button is clicked', async () => {
    const user = userEvent.setup();
    render(<ManualRefundDialog {...defaultProps} />);
    const confirmBtn = screen.getByRole('button', { name: /marcar devolución realizada/i });
    await user.click(confirmBtn);
    expect(defaultProps.onConfirm).toHaveBeenCalledTimes(1);
  });

  it('calls onClose when cancel button is clicked', async () => {
    const user = userEvent.setup();
    render(<ManualRefundDialog {...defaultProps} />);
    const cancelBtn = screen.getByRole('button', { name: /cancelar/i });
    await user.click(cancelBtn);
    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('disables buttons when loading', () => {
    render(<ManualRefundDialog {...defaultProps} isLoading />);
    const buttons = screen.getAllByRole('button');
    buttons.forEach((btn) => {
      expect(btn).toBeDisabled();
    });
  });
});
