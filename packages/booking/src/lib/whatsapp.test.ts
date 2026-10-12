import { describe, expect, it } from 'vitest';
import { whatsappChatHref } from './whatsapp';

describe('whatsappChatHref', () => {
  it('keeps only the digits of a phone written with separators', () => {
    expect(whatsappChatHref('+54 11 5555-0000')).toBe('https://wa.me/541155550000');
  });

  it('adds the prefilled text, encoded, when one is given', () => {
    expect(whatsappChatHref('+541155550000', 'Hola, quiero reservar una cancha.')).toBe(
      'https://wa.me/541155550000?text=Hola%2C%20quiero%20reservar%20una%20cancha.',
    );
  });

  it('leaves the text off when none is given', () => {
    expect(whatsappChatHref('+541155550000')).not.toContain('?');
  });

  it('returns null when the phone has no digits to dial', () => {
    expect(whatsappChatHref('')).toBeNull();
    expect(whatsappChatHref('   ')).toBeNull();
    expect(whatsappChatHref('sin telefono')).toBeNull();
  });
});
