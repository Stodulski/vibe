import { describe, it, expect, vi, afterEach } from 'vitest';
import { render } from '@testing-library/react';

const toasterProps = vi.fn();
vi.mock('sonner', () => ({
  Toaster: (props: { position: string }) => {
    toasterProps(props);
    return null;
  },
}));

function stubViewport(mobile: boolean) {
  vi.spyOn(window, 'matchMedia').mockImplementation(
    (query: string) =>
      ({
        matches: mobile && query === '(max-width: 639px)',
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }) as unknown as MediaQueryList,
  );
}

async function renderProviders() {
  const { Providers } = await import('./providers');
  render(
    <Providers>
      <p>app</p>
    </Providers>,
  );
  return toasterProps.mock.calls.at(-1)?.[0] as { position: string };
}

afterEach(() => {
  vi.restoreAllMocks();
  toasterProps.mockClear();
});

describe('Providers toaster position', () => {
  it('sits at the top on a phone so it never covers a bottom-pinned footer', async () => {
    stubViewport(true);
    expect((await renderProviders()).position).toBe('top-center');
  });

  it('keeps the bottom-right corner on desktop', async () => {
    stubViewport(false);
    expect((await renderProviders()).position).toBe('bottom-right');
  });
});
