import { render, screen, act } from '@testing-library/react';
import { OfflineBanner } from './OfflineBanner';

function setOnline(value: boolean) {
  Object.defineProperty(navigator, 'onLine', {
    value,
    writable: true,
    configurable: true,
  });
}

describe('OfflineBanner', () => {
  const originalOnLine = navigator.onLine;

  afterEach(() => {
    setOnline(originalOnLine);
  });

  it('does not render when online', () => {
    setOnline(true);
    const { container } = render(<OfflineBanner />);
    expect(container.innerHTML).toBe('');
  });

  it('renders banner when offline', () => {
    setOnline(false);
    render(<OfflineBanner />);
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('shows offline text when offline', () => {
    setOnline(false);
    render(<OfflineBanner />);
    expect(screen.getByText(/sin conexi.n/i)).toBeInTheDocument();
  });

  it('shows banner when going offline', () => {
    setOnline(true);
    render(<OfflineBanner />);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();

    act(() => {
      window.dispatchEvent(new Event('offline'));
    });

    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('hides banner when going back online', () => {
    setOnline(false);
    render(<OfflineBanner />);
    expect(screen.getByRole('alert')).toBeInTheDocument();

    act(() => {
      window.dispatchEvent(new Event('online'));
    });

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });
});
