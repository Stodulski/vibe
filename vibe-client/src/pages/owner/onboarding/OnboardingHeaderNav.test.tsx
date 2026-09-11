import { render, screen, fireEvent } from '@testing-library/react';
import { OnboardingHeaderNav } from './OnboardingHeaderNav';

vi.mock('@/shared/components/layout/AppHeader', () => ({
  AppHeader: ({ children }: { children: React.ReactNode }) => <header>{children}</header>,
}));

describe('OnboardingHeaderNav', () => {
  it('shows the back button on step 1 and calls onBack', () => {
    const onBack = vi.fn();
    render(<OnboardingHeaderNav step={1} onBack={onBack} onLogout={vi.fn()} />);
    fireEvent.click(screen.getByRole('button', { name: /Volver/ }));
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it('hides the back button past step 1', () => {
    render(<OnboardingHeaderNav step={2} onBack={vi.fn()} onLogout={vi.fn()} />);
    expect(screen.queryByRole('button', { name: /Volver/ })).not.toBeInTheDocument();
  });

  // jsdom doesn't apply the stylesheet, so `hidden sm:inline` never actually
  // hides its span here the way a real mobile viewport would — this checks
  // the fix's actual shape instead: an `.sr-only` twin carries the label on
  // every viewport, not just above the `sm:` breakpoint.
  it('keeps an always-visible sr-only label alongside the icon on both buttons', () => {
    render(<OnboardingHeaderNav step={1} onBack={vi.fn()} onLogout={vi.fn()} />);
    const backButton = screen.getByRole('button', { name: /Volver/ });
    const logoutButton = screen.getByRole('button', { name: /Cerrar sesión/ });
    expect(backButton.querySelector('.sr-only')).not.toBeNull();
    expect(logoutButton.querySelector('.sr-only')).not.toBeNull();
  });

  it('calls onLogout, a plain callback rather than the mutation object itself', () => {
    const onLogout = vi.fn();
    render(<OnboardingHeaderNav step={1} onBack={vi.fn()} onLogout={onLogout} />);
    fireEvent.click(screen.getByRole('button', { name: /Cerrar sesión/ }));
    expect(onLogout).toHaveBeenCalledTimes(1);
  });
});
