/** The list of pieces: one row each, stacked, never wider than a comfortable reading width. */
export const KIT_LIST_CLASS = 'max-w-3xl space-y-4';

/** Row frame shared by a piece and its loading skeleton, so the two cannot drift. */
export const KIT_ROW_CLASS = 'border-border-subtle bg-bg-elevated flex gap-4 rounded-2xl border p-3 sm:gap-6 sm:p-4';

/** The thumbnail: the post at its 3:4 ratio, a fixed width so every row lines up. */
export const KIT_THUMB_CLASS = 'aspect-[3/4] w-24 shrink-0 self-center overflow-hidden rounded-xl sm:w-36';

/** The two download buttons: stacked on a phone, side by side from `sm`. */
export const KIT_ACTIONS_CLASS = 'flex flex-col gap-2 sm:flex-row';
