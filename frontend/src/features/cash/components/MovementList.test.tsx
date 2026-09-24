import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { makeCashMovement } from '@/test/factories';
import { MovementList } from './MovementList';
import { MovementRow } from './MovementRow';

describe('MovementList', () => {
  it('shows the empty state when there are no movements', () => {
    render(<MovementList movements={[]} onVoid={vi.fn()} />);
    expect(screen.getByText('Todavía no hay movimientos en esta caja')).toBeInTheDocument();
  });

  it('renders every movement, in the given order', () => {
    const movements = [
      makeCashMovement({ id: 'm2', category: 'salaries' }),
      makeCashMovement({ id: 'm1', category: 'supplies' }),
    ];
    render(<MovementList movements={movements} onVoid={vi.fn()} />);
    expect(screen.getByText('Sueldos')).toBeInTheDocument();
    expect(screen.getByText('Insumos')).toBeInTheDocument();
  });

  it('shows Anular on an ordinary, unrelated movement when onVoid is given', () => {
    const movements = [makeCashMovement({ id: 'm1' })];
    render(<MovementList movements={movements} onVoid={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Anular' })).toBeInTheDocument();
  });

  it('hides Anular entirely on a read-only view (no onVoid)', () => {
    const movements = [makeCashMovement({ id: 'm1' })];
    render(<MovementList movements={movements} />);
    expect(screen.queryByRole('button', { name: 'Anular' })).not.toBeInTheDocument();
  });

  it('hides Anular on both rows once one has voided the other — the original is voided, the void cannot be voided', () => {
    const movements = [
      makeCashMovement({ id: 'm1', kind: 'expense', category: 'supplies' }),
      makeCashMovement({ id: 'm2', kind: 'income', category: 'other_income', voids_movement_id: 'm1' }),
    ];
    render(<MovementList movements={movements} onVoid={vi.fn()} />);
    expect(screen.queryByRole('button', { name: 'Anular' })).not.toBeInTheDocument();
  });

  it('badges the voided original as Anulado and labels the void row as an Anulación de <original category>', () => {
    const movements = [
      makeCashMovement({ id: 'm1', kind: 'expense', category: 'supplies' }),
      makeCashMovement({ id: 'm2', kind: 'income', category: 'other_income', voids_movement_id: 'm1' }),
    ];
    render(<MovementList movements={movements} onVoid={vi.fn()} />);
    expect(screen.getByText('Anulado')).toBeInTheDocument();
    expect(screen.getByText(/Anulación de Insumos/)).toBeInTheDocument();
  });

  it('calls onVoid with the movement when Anular is clicked', () => {
    const onVoid = vi.fn();
    const movement = makeCashMovement({ id: 'm1' });
    render(<MovementList movements={[movement]} onVoid={onVoid} />);
    screen.getByRole('button', { name: 'Anular' }).click();
    expect(onVoid).toHaveBeenCalledWith(movement);
  });
});

// System categories (pos-cashbox products/sales delivery): the backend
// writes these on POS sales/restocks, never through the manual movement
// form. A session with one POS sale used to fail to render at all because
// the schema didn't know these values (see cash.schema.test.ts).
describe('MovementList — system categories (sale, restock)', () => {
  it('renders a sale income with the Venta label and no Anular action (backend refuses a manual void with 409)', () => {
    const movements = [makeCashMovement({ id: 's1', kind: 'income', category: 'sale' })];
    render(<MovementList movements={movements} onVoid={vi.fn()} />);
    expect(screen.getByText('Venta')).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Anular' })).not.toBeInTheDocument();
  });

  it('renders a restock expense with the Reposición label and offers Anular (the backend allows a manual void)', () => {
    const movements = [makeCashMovement({ id: 'r1', kind: 'expense', category: 'restock' })];
    render(<MovementList movements={movements} onVoid={vi.fn()} />);
    expect(screen.getByText('Reposición')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Anular' })).toBeInTheDocument();
  });
});

describe('MovementRow — void eligibility in isolation', () => {
  it('never offers Anular on a row that is itself a void, even without its original present', () => {
    const voidRow = makeCashMovement({ id: 'm2', voids_movement_id: 'm1' });
    render(<MovementRow movement={voidRow} movements={[voidRow]} onVoid={vi.fn()} />);
    expect(screen.queryByRole('button', { name: 'Anular' })).not.toBeInTheDocument();
  });

  it('falls back to "Anulación de un movimiento" when the void\'s original is not in the given ledger', () => {
    const voidRow = makeCashMovement({ id: 'm2', voids_movement_id: 'm1' });
    render(<MovementRow movement={voidRow} movements={[voidRow]} onVoid={vi.fn()} />);
    expect(screen.getByText(/Anulación de un movimiento/)).toBeInTheDocument();
  });
});
