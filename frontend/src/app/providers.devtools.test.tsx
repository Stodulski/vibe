import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';

vi.mock('@sentry/react', () => ({ captureException: vi.fn(), setUser: vi.fn() }));
vi.mock('@tanstack/react-query-devtools', () => ({
  ReactQueryDevtools: () => <div data-testid="rq-devtools" />,
}));

async function renderProviders() {
  vi.resetModules();
  const { Providers } = await import('./providers');
  return render(
    <Providers>
      <p>app</p>
    </Providers>,
  );
}

afterEach(() => {
  vi.unstubAllEnvs();
});

// DATA-12: the devtools are worth having in development and must never reach a
// visitor. The guard sits on the dynamic `import()` itself, so in a production
// build Vite drops the branch and the panel is not even a downloadable chunk —
// what this asserts is the observable half of that: whether it mounts at all.
describe('Providers and the React Query devtools', () => {
  it('mounts the devtools in development', async () => {
    vi.stubEnv('DEV', true);
    await renderProviders();
    await waitFor(() => {
      expect(screen.getByTestId('rq-devtools')).toBeInTheDocument();
    });
  });

  it('does not mount them in a production build', async () => {
    vi.stubEnv('DEV', false);
    await renderProviders();
    expect(screen.getByText('app')).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.queryByTestId('rq-devtools')).not.toBeInTheDocument();
    });
  });
});
