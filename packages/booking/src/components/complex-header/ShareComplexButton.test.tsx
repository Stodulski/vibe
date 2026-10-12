import { afterEach, describe, expect, it, vi } from 'vitest';
import { act, fireEvent, screen } from '@testing-library/react';
import { renderBooking } from '../../test/booking';
import { ShareComplexButton } from './ShareComplexButton';
import type { BookingConfig } from '../../config';

const config: BookingConfig = {
  apiBaseUrl: 'http://localhost/api/v1',
  publicSiteUrl: 'https://vibe.com.ar',
  privacyUrl: 'https://vibe.com.ar/privacidad',
};

const canonical = 'https://vibe.com.ar/c/club-padel-norte';
const text = 'Reservá tu cancha en Club Padel Norte';

function setNavigator(properties: { share?: unknown; clipboard?: unknown }) {
  Object.defineProperty(navigator, 'share', { value: properties.share, configurable: true, writable: true });
  Object.defineProperty(navigator, 'clipboard', { value: properties.clipboard, configurable: true, writable: true });
}

function renderShare() {
  return renderBooking(<ShareComplexButton name="Club Padel Norte" slug="club-padel-norte" />, { config });
}

function shareButton() {
  return screen.getByRole('button', { name: 'Compartir Club Padel Norte' });
}

/** Clicks the share button and lets the share chain settle inside act. */
async function clickShare() {
  await act(async () => {
    fireEvent.click(shareButton());
    await Promise.resolve();
  });
}

afterEach(() => {
  Reflect.deleteProperty(navigator, 'share');
  Reflect.deleteProperty(navigator, 'clipboard');
  vi.useRealTimers();
});

describe('ShareComplexButton', () => {
  it('is a named button for the club, visible in the header', () => {
    renderShare();

    expect(shareButton()).toBeInTheDocument();
    expect(shareButton()).toHaveTextContent('Compartir');
  });

  it('shares the club page through navigator.share with the title, text and canonical url', async () => {
    const share = vi.fn().mockResolvedValue(undefined);
    setNavigator({ share });
    renderShare();

    await clickShare();

    expect(share).toHaveBeenCalledWith({ title: 'Club Padel Norte', text, url: canonical });
  });

  it('shows no feedback when the player cancels the native share sheet', async () => {
    const abort = new Error('The user aborted a request.');
    abort.name = 'AbortError';
    const writeText = vi.fn();
    const open = vi.spyOn(window, 'open').mockReturnValue(null);
    setNavigator({ share: vi.fn().mockRejectedValue(abort), clipboard: { writeText } });
    renderShare();

    await clickShare();

    expect(writeText).not.toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
    expect(screen.queryByText('Link copiado')).not.toBeInTheDocument();
  });
});

describe('ShareComplexButton without a native share sheet', () => {
  it('copies the club page link and says so, in a polite live region', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    setNavigator({ clipboard: { writeText } });
    renderShare();

    await clickShare();

    expect(writeText).toHaveBeenCalledWith(canonical);
    expect(screen.getByRole('status')).toHaveTextContent('Link copiado');
    expect(screen.getByRole('status')).toHaveAttribute('aria-live', 'polite');
  });

  it('removes the "Link copiado" feedback after about two seconds', async () => {
    vi.useFakeTimers();
    setNavigator({ clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
    renderShare();

    await clickShare();
    expect(screen.getByRole('status')).toHaveTextContent('Link copiado');

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.queryByText('Link copiado')).not.toBeInTheDocument();
  });

  it('opens WhatsApp with the text and the link when the clipboard rejects', async () => {
    setNavigator({ clipboard: { writeText: vi.fn().mockRejectedValue(new Error('NotAllowedError')) } });
    const open = vi.spyOn(window, 'open').mockReturnValue(null);
    renderShare();

    await clickShare();

    expect(open).toHaveBeenCalledWith(
      `https://wa.me/?text=${encodeURIComponent(`${text} ${canonical}`)}`,
      '_blank',
      'noopener,noreferrer',
    );
    expect(screen.queryByText('Link copiado')).not.toBeInTheDocument();
  });
});
