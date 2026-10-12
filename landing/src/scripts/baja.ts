import { tokenFromSearch, unsubscribe, type UnsubscribeOutcome } from '../lib/unsubscribe.ts';

/** What each answer says to the person. The API's own words are never shown. */
const MESSAGES: Record<UnsubscribeOutcome, string> = {
  done: 'Listo. No vas a recibir más estos mails.',
  invalid: 'El link no es válido. Escribinos a hola@vibe.com.ar y te damos de baja.',
  retry: 'No pudimos darte de baja. Revisá tu conexión y volvé a intentar.',
};

const page = document.querySelector<HTMLElement>('[data-baja]');
const button = page?.querySelector<HTMLButtonElement>('[data-baja-action]');
const status = page?.querySelector<HTMLElement>('[data-baja-status]');

if (page && button && status) {
  const baseUrl = page.dataset.apiUrl ?? '';
  const token = tokenFromSearch(window.location.search);

  const show = (outcome: UnsubscribeOutcome) => {
    status.textContent = MESSAGES[outcome];
    // Only a failed request leaves the button for another try. A link that cannot work, or a
    // done opt-out, would only repeat the same answer, so the button goes.
    button.style.display = outcome === 'retry' ? '' : 'none';
    button.disabled = false;
  };

  // The page only reads the token. Nothing is sent until the person presses the button, so a
  // link scanner or a preview that loads the URL cannot opt anyone out.
  if (token === null) show('invalid');

  button.addEventListener('click', () => {
    button.disabled = true;
    void unsubscribe({ baseUrl, token }).then(show);
  });
}
