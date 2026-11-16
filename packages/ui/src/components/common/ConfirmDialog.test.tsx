import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ConfirmDialog } from './ConfirmDialog';

const defaultProps = {
  open: true,
  onClose: vi.fn(),
  onConfirm: vi.fn(),
  title: 'Delete Item',
  description: 'Are you sure you want to delete this item?',
};

beforeEach(() => {
  vi.clearAllMocks();
});

describe('ConfirmDialog actions', () => {
  it('calls onConfirm when confirm button is clicked', async () => {
    const user = userEvent.setup();
    render(<ConfirmDialog {...defaultProps} />);
    const confirmBtn = screen.getByRole('button', { name: /confirmar/i });
    await user.click(confirmBtn);
    expect(defaultProps.onConfirm).toHaveBeenCalledTimes(1);
  });

  it('calls onClose when cancel button is clicked', async () => {
    const user = userEvent.setup();
    render(<ConfirmDialog {...defaultProps} />);
    const cancelBtn = screen.getByRole('button', { name: /cancelar/i });
    await user.click(cancelBtn);
    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('disables buttons when loading', () => {
    render(<ConfirmDialog {...defaultProps} isLoading />);
    const buttons = screen.getAllByRole('button');
    buttons.forEach((btn) => {
      expect(btn).toBeDisabled();
    });
  });
});

describe('ConfirmDialog content', () => {
  it('renders title and description when open', () => {
    render(<ConfirmDialog {...defaultProps} />);
    expect(screen.getByText('Delete Item')).toBeInTheDocument();
    expect(screen.getByText('Are you sure you want to delete this item?')).toBeInTheDocument();
  });

  it('shows custom confirm label', () => {
    render(<ConfirmDialog {...defaultProps} confirmLabel="Eliminar" />);
    expect(screen.getByRole('button', { name: /eliminar/i })).toBeInTheDocument();
  });

  it('renders children between the description and the buttons', () => {
    render(
      <ConfirmDialog {...defaultProps}>
        <label htmlFor="reason">Motivo</label>
        <textarea id="reason" />
      </ConfirmDialog>,
    );
    const field = screen.getByLabelText('Motivo');
    const confirm = screen.getByRole('button', { name: /confirmar/i });
    expect(field).toBeInTheDocument();
    expect(
      screen.getByText('Are you sure you want to delete this item?').compareDocumentPosition(field) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(field.compareDocumentPosition(confirm) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('uses the destructive button variant for a destructive confirmation', () => {
    render(<ConfirmDialog {...defaultProps} variant="destructive" confirmLabel="Eliminar" />);
    expect(screen.getByRole('button', { name: 'Eliminar' })).toHaveAttribute('data-variant', 'destructive');
  });

  it('gives the title a readable line height so a wrapped title does not collapse', () => {
    render(<ConfirmDialog {...defaultProps} />);
    expect(screen.getByText('Delete Item')).toHaveClass('leading-snug');
    expect(screen.getByText('Delete Item')).not.toHaveClass('leading-none');
  });

});
