import { describe, it, expect } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { BOOKING_MESSAGES } from '../messages';
import { CheckoutLayout } from './CheckoutLayout';

const t = BOOKING_MESSAGES;

function renderLayout() {
  return render(
    <CheckoutLayout
      currentStep={2}
      back={{ label: 'Cambiar horario', href: '/c/club-norte' }}
      aside={<p>Tu horario sigue reservado</p>}
      summary={<section aria-label="Resumen de la reserva">Cancha 1</section>}
    >
      <form aria-label="Tus datos">Campos del formulario</form>
    </CheckoutLayout>,
  );
}

/** The green band: the one element of the checkout that carries the brand background. */
function bandOf(container: HTMLElement): HTMLElement {
  const band = container.querySelector<HTMLElement>('.bg-primary-500');
  if (!band) throw new Error('the checkout has no green band');
  return band;
}

/** True when `later` comes after `earlier` in the document. */
function follows(earlier: Node, later: Node): boolean {
  return (earlier.compareDocumentPosition(later) & Node.DOCUMENT_POSITION_FOLLOWING) !== 0;
}

describe('CheckoutLayout band', () => {
  it('holds the steps, the way back, the aside and the summary in one green band', () => {
    const { container } = renderLayout();
    const band = within(bandOf(container));

    expect(band.getByRole('navigation', { name: t.publicBooking.stepProgress })).toBeInTheDocument();
    expect(band.getByRole('link', { name: 'Cambiar horario' })).toBeInTheDocument();
    expect(band.getByText('Tu horario sigue reservado')).toBeInTheDocument();
    expect(band.getByRole('region', { name: 'Resumen de la reserva' })).toBeInTheDocument();
  });

  // The owner's order: the way back sits at the top, above the steps, as "Volver"
  // does on the complex page's band; then the steps, the countdown, the ticket.
  it('orders the band as way back, steps, aside, then summary', () => {
    const { container } = renderLayout();
    const back = screen.getByRole('link', { name: 'Cambiar horario' });
    const steps = screen.getByRole('navigation', { name: t.publicBooking.stepProgress });
    const aside = screen.getByText('Tu horario sigue reservado');
    const summary = screen.getByRole('region', { name: 'Resumen de la reserva' });

    expect(bandOf(container)).toContainElement(summary);
    expect(follows(back, steps)).toBe(true);
    expect(follows(steps, aside)).toBe(true);
    expect(follows(aside, summary)).toBe(true);
  });

  it('keeps the form below the band, not inside it', () => {
    const { container } = renderLayout();
    const form = screen.getByRole('form', { name: 'Tus datos' });

    expect(bandOf(container)).not.toContainElement(form);
    expect(follows(bandOf(container), form)).toBe(true);
  });

  it('marks the active step in the brand tone', () => {
    renderLayout();
    const active = screen.getByRole('navigation', { name: t.publicBooking.stepProgress }).querySelector('[aria-current="step"]');
    expect(active).not.toBeNull();
  });
});
