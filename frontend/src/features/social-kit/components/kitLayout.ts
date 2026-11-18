import type { SocialKitFormat } from '@/shared/lib/publicPaths';

/** Two cards per piece: stacked on a phone, the story and the post side by side from `sm`. */
export const KIT_PIECE_GRID_CLASS = 'grid max-w-4xl items-start gap-4 sm:grid-cols-2';

/** Card frame shared by a preview and its loading skeleton, so the two cannot drift. */
export const KIT_CARD_CLASS = 'border-border-subtle bg-bg-elevated flex flex-col gap-3 rounded-xl border p-3 sm:p-4';

/** Preview box per format, at the exact aspect ratio of the rendered image. */
export const KIT_PREVIEW_ASPECT_CLASS: Record<SocialKitFormat, string> = {
  historia: 'aspect-[9/16]',
  post: 'aspect-[3/4]',
};
