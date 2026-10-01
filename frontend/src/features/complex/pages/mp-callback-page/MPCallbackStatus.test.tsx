import { describe, it, expect } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { MPCallbackStatus } from './MPCallbackStatus';

const t = ES_AR;

describe('MPCallbackStatus', () => {
  it('offers "Volver a intentar" back to the return path on an expired link', () => {
    renderWithProviders(<MPCallbackStatus status="error" errorReason="expired" returnPath="/onboarding" />);

    expect(screen.getByText(t.mp.connectErrorExpired)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: t.mp.connectRetry })).toHaveAttribute('href', '/onboarding');
  });

  it('explains the conflict and only offers the way back', () => {
    renderWithProviders(<MPCallbackStatus status="error" errorReason="conflict" returnPath="/settings" />);

    expect(screen.getByText(t.mp.connectErrorConflict)).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: t.mp.connectRetry })).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: t.common.back })).toHaveAttribute('href', '/settings');
  });

  it('shows the generic message with retry for any other failure', () => {
    renderWithProviders(<MPCallbackStatus status="error" errorReason="failed" returnPath="/settings" />);

    expect(screen.getByText(t.mp.connectError)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: t.mp.connectRetry })).toBeInTheDocument();
  });

  it('shows no action while processing or after success', () => {
    const { rerender } = renderWithProviders(
      <MPCallbackStatus status="processing" errorReason="expired" returnPath="/settings" />,
    );
    expect(screen.queryByRole('link')).not.toBeInTheDocument();

    rerender(<MPCallbackStatus status="success" errorReason="expired" returnPath="/settings" />);
    expect(screen.getByText(t.mp.connectSuccess)).toBeInTheDocument();
  });
});
