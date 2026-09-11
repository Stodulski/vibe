import { render, screen } from '@testing-library/react';
import { StepIndicator } from './StepIndicator';

describe('StepIndicator', () => {
  it('renders steps as a semantic ordered list inside the nav', () => {
    render(
      <StepIndicator currentStep={2} totalSteps={3} stepLabels={['Datos', 'Pago', 'Confirmar']} ariaLabel="Progreso" />,
    );

    const nav = screen.getByRole('navigation', { name: 'Progreso' });
    const list = screen.getByRole('list');
    expect(nav).toContainElement(list);
    expect(screen.getAllByRole('listitem')).toHaveLength(3);
  });

  it('marks only the current step with aria-current="step"', () => {
    render(
      <StepIndicator currentStep={2} totalSteps={3} stepLabels={['Datos', 'Pago', 'Confirmar']} ariaLabel="Progreso" />,
    );

    const items = screen.getAllByRole('listitem');
    expect(items[0]).not.toHaveAttribute('aria-current');
    expect(items[1]).toHaveAttribute('aria-current', 'step');
    expect(items[2]).not.toHaveAttribute('aria-current');
  });
});
