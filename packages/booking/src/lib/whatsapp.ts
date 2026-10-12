/**
 * A WhatsApp chat with a number, as a `wa.me` link.
 *
 * `wa.me` takes the number as digits only, with the country code and no `+`,
 * so every separator a club types into its phone field is dropped here. Null
 * when no digits are left, so a club without a usable number gets no link at
 * all rather than a broken one. The text, when given, is prefilled and encoded.
 */
export function whatsappChatHref(phone: string, text?: string): string | null {
  const digits = phone.replace(/\D/g, '');
  if (digits === '') return null;

  const base = `https://wa.me/${digits}`;
  return text ? `${base}?text=${encodeURIComponent(text)}` : base;
}
