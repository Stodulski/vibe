import { render, screen } from '@testing-library/react';
import { StepIndicator } from './StepIndicator';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

describe('StepIndicator', () => {
  it('marks the active step with aria-current="step"', () => {
    render(<StepIndicator currentStep={2} />);
    expect(screen.getByText(t.publicBooking.stepData).closest('[aria-current="step"]')).not.toBeNull();
  });

  // The circle labels are `sm:inline` only — on a phone the row of circles
  // alone cannot answer "which step am I on", so the mobile-only line under
  // them carries the active step's own label.
  it('shows the active step number and label on the mobile-only line', () => {
    render(<StepIndicator currentStep={2} />);
    expect(
      screen.getByText(
        `${t.publicBooking.stepOfPrefix} 2 ${t.publicBooking.stepOfMiddle} 3: ${t.publicBooking.stepData}`,
      ),
    ).toBeInTheDocument();
  });

  it('updates the mobile-only line for the first step', () => {
    render(<StepIndicator currentStep={1} />);
    expect(
      screen.getByText(
        `${t.publicBooking.stepOfPrefix} 1 ${t.publicBooking.stepOfMiddle} 3: ${t.publicBooking.stepSelect}`,
      ),
    ).toBeInTheDocument();
  });

  it('updates the mobile-only line for the last step', () => {
    render(<StepIndicator currentStep={3} />);
    expect(
      screen.getByText(
        `${t.publicBooking.stepOfPrefix} 3 ${t.publicBooking.stepOfMiddle} 3: ${t.publicBooking.stepPay}`,
      ),
    ).toBeInTheDocument();
  });
});
