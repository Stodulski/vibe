import { render, screen } from '@testing-library/react';
import { OnboardingStepIndicator } from './OnboardingStepIndicator';

vi.mock('./useOnboarding', () => ({}));

describe('OnboardingStepIndicator', () => {
  it('renders 3 steps', () => {
    render(<OnboardingStepIndicator currentStep={1} />);
    // Should render short labels for mobile: Complejo, Canchas, Pagos
    expect(screen.getByText('Complejo')).toBeInTheDocument();
    expect(screen.getByText('Canchas')).toBeInTheDocument();
    expect(screen.getByText('Pagos')).toBeInTheDocument();
  });

  it('renders desktop labels', () => {
    render(<OnboardingStepIndicator currentStep={1} />);
    expect(screen.getByText('Datos del complejo')).toBeInTheDocument();
    expect(screen.getByText('Primera cancha')).toBeInTheDocument();
    expect(screen.getByText('Cobros online')).toBeInTheDocument();
  });

  it('marks only the active step with aria-current="step", for screen readers', () => {
    render(<OnboardingStepIndicator currentStep={2} />);
    const active = document.querySelector('[aria-current="step"]');
    expect(active).toContainElement(screen.getByText('Canchas'));
    expect(active).not.toContainElement(screen.getByText('Complejo'));
    expect(active).not.toContainElement(screen.getByText('Pagos'));
    expect(document.querySelectorAll('[aria-current="step"]')).toHaveLength(1);
  });

  it('labels the step sequence as a landmark', () => {
    render(<OnboardingStepIndicator currentStep={1} />);
    expect(screen.getByRole('navigation', { name: /progreso/i })).toBeInTheDocument();
  });
});
