/** The pieces as a gallery: one card per piece, two side by side from `sm`. */
export const KIT_LIST_CLASS = 'grid max-w-3xl gap-4 sm:grid-cols-2 sm:gap-6';

/** Card frame shared by a piece and its loading skeleton, so the two cannot drift. */
export const KIT_ROW_CLASS = 'border-border-subtle bg-bg-elevated flex flex-col gap-4 rounded-2xl border p-3 sm:p-4';

/** The preview: the post at its 3:4 ratio, as wide as the card. */
export const KIT_THUMB_CLASS = 'aspect-[3/4] w-full overflow-hidden rounded-xl';

/** The two download buttons, stacked at the card's full width. */
export const KIT_ACTIONS_CLASS = 'flex flex-col gap-2';
