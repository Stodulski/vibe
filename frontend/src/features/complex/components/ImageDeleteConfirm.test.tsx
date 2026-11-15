import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ImageDeleteConfirm } from './ImageDeleteConfirm';

async function openDialog() {
  const user = userEvent.setup();
  const onConfirm = vi.fn();
  render(<ImageDeleteConfirm disabled={false} onConfirm={onConfirm} />);
  await user.click(screen.getByRole('button'));
  return { user, onConfirm };
}

describe('ImageDeleteConfirm', () => {
  it('confirms deletion with the destructive variant', async () => {
    await openDialog();
    expect(screen.getByRole('button', { name: 'Eliminar', exact: true })).toHaveAttribute(
      'data-variant',
      'destructive',
    );
  });

  it('calls onConfirm when the owner confirms', async () => {
    const { user, onConfirm } = await openDialog();
    await user.click(screen.getByRole('button', { name: 'Eliminar', exact: true }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('dismisses with Cancelar and does not call onConfirm', async () => {
    const { user, onConfirm } = await openDialog();
    await user.click(screen.getByRole('button', { name: 'Cancelar' }));
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
