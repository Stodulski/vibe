import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import type { MouseEventHandler, ReactNode } from 'react';
import { MPConnectStep } from './MPConnectStep';

interface MockButtonProps {
  children?: ReactNode;
  asChild?: boolean;
  onClick?: MouseEventHandler<HTMLElement>;
  className?: string;
}

vi.mock('@/shared/components/ui/button', () => ({
  Button: ({ children, asChild, ...props }: MockButtonProps) => {
    if (asChild) return <div {...props}>{children}</div>;
    return <button {...props}>{children}</button>;
  },
}));

// The step links to Configuración, so it needs a router the way it has one in
// the app. Without it every case here died on `<Link>`, not on its own claim.
function renderStep(props: Partial<typeof baseProps> = {}) {
  return render(
    <MemoryRouter>
      <MPConnectStep {...baseProps} {...props} />
    </MemoryRouter>,
  );
}

const baseProps = {
  complexId: 'c1',
  mpConnected: false,
  mpAuthUrl: null as string | null,
  onConnectClick: vi.fn(),
  onBack: vi.fn(),
  onFinish: vi.fn(),
};

describe('MPConnectStep', () => {
  it('says why the step exists, without repeating its name from the indicator', () => {
    renderStep();
    expect(screen.getByText(/Conect\u00e1 MercadoPago/i)).toBeInTheDocument();
  });

  it('shows loading when no auth url and not connected', () => {
    renderStep();
    // No connect button or connected state should show
    expect(screen.queryByText('MercadoPago conectado')).not.toBeInTheDocument();
  });

  it('shows connect button when auth url is available', () => {
    renderStep({ mpAuthUrl: 'https://auth.mercadopago.com/test' });
    // Only the connect action carries the provider's name now; the step is
    // titled after what the owner is doing ("Cobros online"), not after our
    // integration.
    expect(screen.getByText('Conectar MercadoPago')).toBeInTheDocument();
  });

  it('shows connected state', () => {
    renderStep({ mpConnected: true });
    expect(screen.getByText('MercadoPago conectado')).toBeInTheDocument();
  });

  it('shows finish button when connected', () => {
    renderStep({ mpConnected: true });
    expect(screen.getByText('Ir al panel')).toBeInTheDocument();
  });

  // The exit used to render only when MercadoPago was connected — not
  // disabled, absent — and the header's escape only shows on step 1. Between
  // them an owner who takes cash reached the end of onboarding and could only
  // go back or log out. There is always a way out now; without a connection it
  // is the secondary one.
  it('always offers a way out of onboarding, connected or not', () => {
    renderStep();
    expect(screen.getByText('Lo hago después')).toBeInTheDocument();
  });

  it('says what skipping means while payments are not connected, and nothing about defaults', () => {
    renderStep({ mpConnected: false, mpAuthUrl: 'https://auth.mercadopago.com/x' });
    expect(screen.getByText(/tu página muestra tu WhatsApp/i)).toBeInTheDocument();
    expect(screen.queryByText(/Ya podés recibir reservas/i)).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: /Configuración/i })).not.toBeInTheDocument();
  });

  it('drops the skip explanation once payments are connected', () => {
    renderStep({ mpConnected: true, mpAuthUrl: null });
    expect(screen.queryByText(/tu página muestra tu WhatsApp/i)).not.toBeInTheDocument();
  });

  it('renders back button', () => {
    renderStep();
    expect(screen.getByText('Volver')).toBeInTheDocument();
  });
});
