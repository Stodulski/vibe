import type { SocialKitFormat } from '@/shared/lib/publicPaths';

/** The two pieces side by side in the chosen format, so both cards always have the same shape. */
export const KIT_GRID_CLASS = 'grid max-w-3xl grid-cols-2 gap-3 sm:gap-6';

/** Card frame shared by a preview and its loading skeleton, so the two cannot drift. */
export const KIT_CARD_CLASS =
  'border-border-subtle bg-bg-elevated flex flex-col gap-3 rounded-2xl border p-2 sm:gap-4 sm:p-4';

/** Preview box per format, at the exact aspect ratio of the rendered image. */
export const KIT_PREVIEW_ASPECT_CLASS: Record<SocialKitFormat, string> = {
  historia: 'aspect-[9/16]',
  post: 'aspect-[3/4]',
};

/** The format switch: the app's segmented pair, two 40px toggles sharing the width on a phone. */
export const KIT_SWITCH_CLASS = 'grid max-w-sm grid-cols-2 gap-2';
export const KIT_SWITCH_SEGMENT_CLASS = 'h-10 min-w-0 rounded-xl border px-4 whitespace-nowrap';
