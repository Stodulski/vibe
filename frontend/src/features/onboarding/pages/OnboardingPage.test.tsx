import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';

const useOnboardingMock = vi.fn();

vi.mock('./onboarding/useOnboarding', () => ({
  useOnboarding: () => useOnboardingMock() as unknown,
}));

vi.mock('@/shared/components/common/ComplexLoadError', () => ({
  ComplexLoadError: ({ onRetry }: { onRetry: () => void }) => (
    <div data-testid="complex-load-error">
      <button onClick={onRetry}>retry</button>
    </div>
  ),
}));

vi.mock('./onboarding/OnboardingStepIndicator', () => ({
  OnboardingStepIndicator: () => <div data-testid="step-indicator" />,
}));

vi.mock('./onboarding/OnboardingHeaderNav', () => ({
  OnboardingHeaderNav: () => <div data-testid="header-nav" />,
}));

vi.mock('./onboarding/OnboardingStepContent', () => ({
  OnboardingStepContent: () => <div data-testid="step-content" />,
}));

describe('OnboardingPage', () => {
  it('shows a retryable error instead of step 1 when the complexes query errored', async () => {
    const refetchComplexes = vi.fn();
    useOnboardingMock.mockReturnValue({
      step: 1,
      animKey: 0,
      logout: { mutate: vi.fn() },
      complexesError: true,
      refetchComplexes,
    });

    const OnboardingPage = (await import('./OnboardingPage')).default;
    render(<OnboardingPage />);

    expect(screen.getByTestId('complex-load-error')).toBeInTheDocument();
    expect(screen.queryByTestId('step-content')).not.toBeInTheDocument();

    screen.getByRole('button', { name: 'retry' }).click();
    expect(refetchComplexes).toHaveBeenCalled();
  });

  it('renders the step content once the account has no complex yet (a genuinely empty, successful query)', async () => {
    useOnboardingMock.mockReturnValue({
      step: 1,
      animKey: 0,
      logout: { mutate: vi.fn() },
      complexesError: false,
      refetchComplexes: vi.fn(),
    });

    const OnboardingPage = (await import('./OnboardingPage')).default;
    render(<OnboardingPage />);

    expect(screen.queryByTestId('complex-load-error')).not.toBeInTheDocument();
    expect(screen.getByTestId('step-content')).toBeInTheDocument();
  });

  it('keeps rendering step content when a background refetch fails but a complex is still cached', async () => {
    useOnboardingMock.mockReturnValue({
      step: 2,
      animKey: 0,
      logout: { mutate: vi.fn() },
      complexesError: true,
      complexesFetching: false,
      refetchComplexes: vi.fn(),
      currentComplex: { id: 'c1', mp_user_id: null },
    });

    const OnboardingPage = (await import('./OnboardingPage')).default;
    render(<OnboardingPage />);

    expect(screen.queryByTestId('complex-load-error')).not.toBeInTheDocument();
    expect(screen.getByTestId('step-content')).toBeInTheDocument();
  });
});
