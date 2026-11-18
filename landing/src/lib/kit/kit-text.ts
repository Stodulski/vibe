/**
 * The words of the kit pictures: the headlines, the steps and the link each piece shows, in the
 * Spanish of the kit brief. Pure, so node:test can run it. The layout that places these words
 * lives in kit-tree.ts.
 */
import type { ShareFacts } from '../complex-share.ts';
import { complexPath } from '../hub.ts';
import { complexCardText } from '../og/complex-card.ts';
import { STOREFRONT_ORIGIN } from '../storefront.ts';
import type { KitPiece } from './kit-image.ts';

export interface KitText {
  /** The complex name, normalized. */
  name: string;
  /** The complex page on the storefront origin: what the QR code encodes. */
  url: string;
  /** The same page without the scheme, as the picture shows it. */
  link: string;
  headline: string;
  /** The line above the link on reservar. Empty on como-reservar. */
  scanLine: string;
  /** The sports joined with middle dots on reservar. Empty when there is none, and on como-reservar. */
  sports: string;
  /** The three steps of como-reservar. Empty on reservar. */
  steps: readonly string[];
}

export function kitText(facts: ShareFacts, slug: string, piece: KitPiece): KitText {
  const url = `${STOREFRONT_ORIGIN}${complexPath(slug)}`;
  const link = url.replace(/^https:\/\//, '');
  const { name, sports } = complexCardText(facts);
  if (piece === 'reservar') {
    return {
      name,
      url,
      link,
      headline: 'Reservá tu cancha online',
      scanLine: 'Escaneá el código o entrá a',
      sports,
      steps: [],
    };
  }
  return {
    name,
    url,
    link,
    headline: 'Reservá en 3 pasos',
    scanLine: '',
    sports: '',
    steps: [`Entrá a ${link}`, 'Elegí deporte, horario y cancha', 'Confirmá y te llega el turno por email'],
  };
}
