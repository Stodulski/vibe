import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { OnboardingHeaderNav } from './OnboardingHeaderNav';

vi.mock('@/shared/components/layout/AppHeader', () => ({
  AppHeader: ({ children }: { children: React.ReactNode }) => <header>{children}</header>,
}));

describe('OnboardingHeaderNav', () => {
  it('has no back button — the account owns a single complex, so step 1 has nowhere to go back to', () => {
    render(<OnboardingHeaderNav onLogout={vi.fn()} />);
    expect(screen.queryByRole('button', { name: /Volver/ })).not.toBeInTheDocument();
  });

  // jsdom doesn't apply the stylesheet, so `hidden sm:inline` never actually
  // hides its span here the way a real mobile viewport would — this checks
  // the fix's actual shape instead: an `.sr-only` twin carries the label on
  // every viewport, not just above the `sm:` breakpoint.
  it('keeps an always-visible sr-only label alongside the icon', () => {
    render(<OnboardingHeaderNav onLogout={vi.fn()} />);
    const logoutButton = screen.getByRole('button', { name: /Cerrar sesión/ });
    expect(logoutButton.querySelector('.sr-only')).not.toBeNull();
  });

  it('calls onLogout, a plain callback rather than the mutation object itself', () => {
    const onLogout = vi.fn();
    render(<OnboardingHeaderNav onLogout={onLogout} />);
    fireEvent.click(screen.getByRole('button', { name: /Cerrar sesión/ }));
    expect(onLogout).toHaveBeenCalledTimes(1);
  });
});
