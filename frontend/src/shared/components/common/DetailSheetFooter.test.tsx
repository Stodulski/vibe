import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Sheet, SheetContent, SheetTitle } from '@/shared/components/ui/sheet';
import { Button } from '@/shared/components/ui/button';
import { DetailSheetFooter } from './DetailSheetFooter';

function renderFooter(ui: React.ReactNode) {
  render(
    <Sheet open>
      <SheetContent showCloseButton={false}>
        <SheetTitle>Detalle</SheetTitle>
        {ui}
      </SheetContent>
    </Sheet>,
  );
}

describe('DetailSheetFooter', () => {
  it('pins its action in a sheet footer with the body padding, full width on mobile and right-aligned from sm', () => {
    renderFooter(
      <DetailSheetFooter>
        <Button>Confirmar pago</Button>
      </DetailSheetFooter>,
    );

    const footer = screen.getByRole('button', { name: 'Confirmar pago' }).parentElement;
    expect(footer).toHaveAttribute('data-slot', 'sheet-footer');
    expect(footer).toHaveClass('sm:px-6', 'sm:items-end');
    expect(footer?.className).toContain('[&>button]:w-full');
    expect(footer?.className).toContain('sm:[&>button]:w-auto');
  });

  it('renders nothing without an action', () => {
    renderFooter(<DetailSheetFooter>{null}</DetailSheetFooter>);

    expect(document.querySelector('[data-slot="sheet-footer"]')).toBeNull();
  });

  it('keeps the action full width on every size when asked to (bottom sheets)', () => {
    renderFooter(
      <DetailSheetFooter fullWidth>
        <Button>Cobrar</Button>
      </DetailSheetFooter>,
    );

    const footer = screen.getByRole('button', { name: 'Cobrar' }).parentElement;
    expect(footer).not.toHaveClass('sm:items-end');
    expect(footer?.className).not.toContain('sm:[&>button]:w-auto');
  });

  it('puts the destructive action on the left and the primary on the right, stacked with the primary on top on mobile', () => {
    renderFooter(
      <DetailSheetFooter secondary={<Button variant="destructive-outline">Cancelar evento</Button>}>
        <Button>Confirmar pago</Button>
      </DetailSheetFooter>,
    );

    const destructive = screen.getByRole('button', { name: 'Cancelar evento' });
    const footer = destructive.parentElement;
    expect(destructive).toHaveAttribute('data-variant', 'destructive-outline');
    expect(footer).toHaveClass('px-4', 'flex-col-reverse', 'sm:flex-row', 'sm:justify-between');
    expect(footer?.firstElementChild).toBe(destructive);
    expect(footer?.lastElementChild).toBe(screen.getByRole('button', { name: 'Confirmar pago' }));
  });

  it('renders a lone destructive action with no primary', () => {
    renderFooter(<DetailSheetFooter secondary={<Button variant="destructive-outline">Desbloquear</Button>} />);

    expect(screen.getByRole('button', { name: 'Desbloquear' }).parentElement).toHaveAttribute(
      'data-slot',
      'sheet-footer',
    );
  });
});
