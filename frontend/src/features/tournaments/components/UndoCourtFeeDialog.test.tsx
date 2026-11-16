import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { UndoCourtFeeDialog } from './UndoCourtFeeDialog';

describe('UndoCourtFeeDialog', () => {
  it('dismisses with Cancelar, since its primary does not start with Cancelar or Anular', () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <UndoCourtFeeDialog
          open
          onClose={vi.fn()}
          complexId="c1"
          tournamentId="t1"
          target={null}
          pairName="Pareja"
          amount={1000}
        />
      </QueryClientProvider>,
    );
    expect(screen.getByRole('button', { name: 'Cancelar' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Deshacer cobro' })).toHaveAttribute('data-variant', 'destructive');
  });
});
