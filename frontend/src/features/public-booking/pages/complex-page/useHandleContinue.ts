import type { SelectedSlot } from '@/features/public-booking';
import type { PublicComplex } from '@/shared/types/api.types';
import { publicComplexPath } from '@/shared/lib/publicPaths';
import { navigateTo } from '../../lib/navigation';
import { saveConfirmDraft } from '../../lib/handoff';
import { buildConfirmState } from './buildConfirmState';

export function useHandleContinue(
  slug: string | undefined,
  complex: PublicComplex | undefined,
  mpConnected: boolean,
  selectedSlot: SelectedSlot | null,
  dateStr: string,
) {
  /**
   * Off to the confirm page with a selection.
   *
   * The selection can be handed in directly. Choosing a court continues in the
   * same tap, and at that moment the page's `selectedSlot` is still the
   * previous render's value — reading it here would navigate with nothing.
   * With no argument, the settled selection is used, as from the Continue
   * button under the hours.
   *
   * The selection travels as a draft in sessionStorage, keyed by slug, because
   * the confirm page is a separate document.
   */
  return function handleContinue(selection: SelectedSlot | null = selectedSlot) {
    if (!complex || !selection || !mpConnected) return;
    saveConfirmDraft(String(slug), buildConfirmState(complex, selection, dateStr));
    navigateTo(`${publicComplexPath(String(slug))}/book/confirm`);
  };
}
