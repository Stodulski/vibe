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
  it('puts the destructive action first and the primary last', () => {
    renderFooter(
      <DetailSheetFooter secondary={<Button variant="destructive-outline">Cancelar evento</Button>}>
        <Button>Confirmar pago</Button>
      </DetailSheetFooter>,
    );

    const destructive = screen.getByRole('button', { name: 'Cancelar evento' });
    const footer = destructive.parentElement;
    expect(footer?.firstElementChild).toBe(destructive);
    expect(footer?.lastElementChild).toBe(screen.getByRole('button', { name: 'Confirmar pago' }));
  });

  it('renders a lone destructive action with no primary', () => {
    renderFooter(<DetailSheetFooter secondary={<Button variant="destructive-outline">Desbloquear</Button>} />);

    expect(screen.getByRole('button', { name: 'Desbloquear' })).toBeInTheDocument();
  });
});
