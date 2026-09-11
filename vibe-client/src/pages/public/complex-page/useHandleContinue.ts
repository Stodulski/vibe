import { useNavigate } from 'react-router-dom';
import type { SelectedSlot } from '@/features/public-booking';
import type { PublicComplex } from '@/shared/types/api.types';
import { buildConfirmState } from './buildConfirmState';

export function useHandleContinue(
  slug: string | undefined,
  complex: PublicComplex | undefined,
  mpConnected: boolean,
  selectedSlot: SelectedSlot | null,
  dateStr: string,
) {
  const navigate = useNavigate();

  /**
   * Off to the confirm page with a selection.
   *
   * The selection can be handed in directly. Choosing a court continues in the
   * same tap, and at that moment the page's `selectedSlot` is still the
   * previous render's value — reading it here would navigate with nothing.
   * With no argument, the settled selection is used, as from the Continue
   * button under the hours.
   */
  return function handleContinue(selection: SelectedSlot | null = selectedSlot) {
    if (!complex || !selection || !mpConnected) return;
    void navigate(`/${String(slug)}/book/confirm`, {
      state: buildConfirmState(complex, selection, dateStr),
    });
  };
}
