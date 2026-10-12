import { describe, expect, it, vi } from 'vitest';
import { shareComplexPage, whatsappShareHref, type SharePayload } from './shareComplex';

const payload: SharePayload = {
  title: 'Club Padel Norte',
  text: 'Reservá tu cancha en Club Padel Norte',
  url: 'https://vibe.com.ar/c/club-padel-norte',
};

function abortError(): Error {
  const error = new Error('The user aborted a request.');
  error.name = 'AbortError';
  return error;
}

describe('shareComplexPage', () => {
  it('hands the title, text and url to navigator.share', async () => {
    const share = vi.fn().mockResolvedValue(undefined);
    const copy = vi.fn();
    const openWhatsApp = vi.fn();

    const outcome = await shareComplexPage(payload, { share, copy, openWhatsApp });

    expect(share).toHaveBeenCalledWith({ title: payload.title, text: payload.text, url: payload.url });
    expect(outcome).toBe('shared');
    expect(copy).not.toHaveBeenCalled();
    expect(openWhatsApp).not.toHaveBeenCalled();
  });

  it('copies the url when there is no native share, and reports it as copied', async () => {
    const copy = vi.fn().mockResolvedValue(undefined);
    const openWhatsApp = vi.fn();

    const outcome = await shareComplexPage(payload, { copy, openWhatsApp });

    expect(copy).toHaveBeenCalledWith(payload.url);
    expect(outcome).toBe('copied');
    expect(openWhatsApp).not.toHaveBeenCalled();
  });

  it('opens WhatsApp with the text and the url when the clipboard rejects', async () => {
    const copy = vi.fn().mockRejectedValue(new Error('NotAllowedError'));
    const openWhatsApp = vi.fn();

    const outcome = await shareComplexPage(payload, { copy, openWhatsApp });

    expect(openWhatsApp).toHaveBeenCalledWith(whatsappShareHref(payload));
    expect(outcome).toBe('whatsapp');
  });

  it('opens WhatsApp when there is no clipboard at all', async () => {
    const openWhatsApp = vi.fn();

    const outcome = await shareComplexPage(payload, { openWhatsApp });

    expect(openWhatsApp).toHaveBeenCalledWith(whatsappShareHref(payload));
    expect(outcome).toBe('whatsapp');
  });

  it('ignores a native share the player cancelled', async () => {
    const share = vi.fn().mockRejectedValue(abortError());
    const copy = vi.fn();
    const openWhatsApp = vi.fn();

    const outcome = await shareComplexPage(payload, { share, copy, openWhatsApp });

    expect(outcome).toBe('cancelled');
    expect(copy).not.toHaveBeenCalled();
    expect(openWhatsApp).not.toHaveBeenCalled();
  });

  it('falls back to the clipboard when the native share fails for another reason', async () => {
    const share = vi.fn().mockRejectedValue(new Error('NotAllowedError'));
    const copy = vi.fn().mockResolvedValue(undefined);
    const openWhatsApp = vi.fn();

    const outcome = await shareComplexPage(payload, { share, copy, openWhatsApp });

    expect(copy).toHaveBeenCalledWith(payload.url);
    expect(outcome).toBe('copied');
    expect(openWhatsApp).not.toHaveBeenCalled();
  });
});

describe('whatsappShareHref', () => {
  it('builds a wa.me link without a number, with the text and the url encoded', () => {
    expect(whatsappShareHref(payload)).toBe(
      `https://wa.me/?text=${encodeURIComponent(`${payload.text} ${payload.url}`)}`,
    );
  });
});
