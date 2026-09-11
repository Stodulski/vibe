import { render, screen, fireEvent } from '@testing-library/react';
import { useLoadMoreOnScroll } from './useLoadMoreOnScroll';

function TestStrip({ onNearEnd }: { onNearEnd: () => void }) {
  const scrollRef = useLoadMoreOnScroll(onNearEnd);
  return <div ref={scrollRef} data-testid="strip" />;
}

function makeNearEnd(el: HTMLElement) {
  Object.defineProperty(el, 'scrollLeft', { value: 1000, configurable: true });
  Object.defineProperty(el, 'clientWidth', { value: 100, configurable: true });
  Object.defineProperty(el, 'scrollWidth', { value: 1050, configurable: true });
}

describe('useLoadMoreOnScroll', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('calls the latest onNearEnd after a prop change, without re-attaching the scroll listener', () => {
    const first = vi.fn();
    const second = vi.fn();
    const { rerender } = render(<TestStrip onNearEnd={first} />);
    const el = screen.getByTestId('strip');
    makeNearEnd(el);

    // Swap the callback the way a re-render with a fresh inline function
    // would — the effect that attaches the listener only ever runs once
    // (mount), so the fix has to reach the new callback through a ref, not
    // by re-subscribing.
    rerender(<TestStrip onNearEnd={second} />);

    fireEvent.scroll(el);
    vi.advanceTimersByTime(300);

    expect(second).toHaveBeenCalledTimes(1);
    expect(first).not.toHaveBeenCalled();
  });

  it('debounces rapid scroll events into a single onNearEnd call', () => {
    const onNearEnd = vi.fn();
    render(<TestStrip onNearEnd={onNearEnd} />);
    const el = screen.getByTestId('strip');
    makeNearEnd(el);

    fireEvent.scroll(el);
    vi.advanceTimersByTime(100);
    fireEvent.scroll(el);
    vi.advanceTimersByTime(300);

    expect(onNearEnd).toHaveBeenCalledTimes(1);
  });
});
