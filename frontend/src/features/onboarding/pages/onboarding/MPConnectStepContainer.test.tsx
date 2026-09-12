import { describe, it, expect, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { MPConnectStepContainer } from './MPConnectStepContainer';

interface MockButtonProps {
  children?: React.ReactNode;
  asChild?: boolean;
  onClick?: React.MouseEventHandler<HTMLElement>;
}

vi.mock('@/shared/components/ui/button', () => ({
  Button: ({ children, asChild, ...props }: MockButtonProps) => {
    if (asChild) return <div {...props}>{children}</div>;
    return <button {...props}>{children}</button>;
  },
}));

const useMPConnectMock = vi.fn();
vi.mock('@/features/complex', () => ({
  useMPConnect: (...args: unknown[]) => useMPConnectMock(...args) as unknown,
}));

describe('MPConnectStepContainer', () => {
  it('passes the hook state into the presentational step, polling every 3s', () => {
    useMPConnectMock.mockReturnValue({
      connected: false,
      authUrl: 'https://mp.test/auth',
      handleConnectClick: vi.fn(),
    });
    render(
      <MemoryRouter>
        <MPConnectStepContainer complexId="c1" completeOnboarding={vi.fn()} onBack={vi.fn()} />
      </MemoryRouter>,
    );
    expect(useMPConnectMock).toHaveBeenCalledWith('c1', { refetchInterval: 3000 });
    expect(screen.getByText('Conectar MercadoPago')).toBeInTheDocument();
  });

  it('completes onboarding once the status flips from disconnected to connected', () => {
    useMPConnectMock.mockReturnValue({
      connected: false,
      authUrl: 'https://mp.test/auth',
      handleConnectClick: vi.fn(),
    });
    const completeOnboarding = vi.fn();
    const { rerender } = render(
      <MemoryRouter>
        <MPConnectStepContainer complexId="c1" completeOnboarding={completeOnboarding} onBack={vi.fn()} />
      </MemoryRouter>,
    );
    expect(completeOnboarding).not.toHaveBeenCalled();

    useMPConnectMock.mockReturnValue({
      connected: true,
      authUrl: 'https://mp.test/auth',
      handleConnectClick: vi.fn(),
    });
    rerender(
      <MemoryRouter>
        <MPConnectStepContainer complexId="c1" completeOnboarding={completeOnboarding} onBack={vi.fn()} />
      </MemoryRouter>,
    );
    expect(completeOnboarding).toHaveBeenCalledWith('c1');
  });

  it('calls completeOnboarding with the complex id when the finish action fires', () => {
    useMPConnectMock.mockReturnValue({
      connected: true,
      authUrl: null,
      handleConnectClick: vi.fn(),
    });
    const completeOnboarding = vi.fn();
    render(
      <MemoryRouter>
        <MPConnectStepContainer complexId="c1" completeOnboarding={completeOnboarding} onBack={vi.fn()} />
      </MemoryRouter>,
    );
    fireEvent.click(screen.getByText('Ir al panel'));
    expect(completeOnboarding).toHaveBeenCalledWith('c1');
  });
});
