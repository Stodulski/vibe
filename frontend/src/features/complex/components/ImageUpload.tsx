import { Upload, Camera, ImageIcon } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import type { Complex } from '@/shared/types/api.types';
import { useImageSlot } from './useImageSlot';
import { ImageDeleteConfirm } from './ImageDeleteConfirm';

const t = ES_AR;

const ACCEPTED = 'image/jpeg,image/png,image/webp';

/**
 * The bounds `compressImage` caps each kind of upload to
 * (LOGO_OPTIONS/COVER_OPTIONS), which is what a preview's intrinsic size
 * is: it tells the browser the ratio to reserve before the file loads
 * (PERF-08).
 */
const PREVIEW_SIZE = {
  logo: { width: 512, height: 512 },
  cover: { width: 1280, height: 720 },
} as const;

/**
 * The logo and the cover, arranged the way the public page arranges them.
 *
 * They were two bordered cards stacked on top of each other — one container
 * per image, for two images that are never seen apart. The cover is the banner
 * and the logo sits over its bottom-left corner, which is exactly what a
 * visitor sees, so the editor is a picture of its own result rather than a
 * pair of file pickers that happen to be adjacent.
 *
 * The images ARE the buttons. A separate "Subir logo" beside a logo slot that
 * already opens the file picker is the same action twice; the empty slot shows
 * an upload icon and the filled one shows it on hover. Deleting still needs
 * its own control, because nothing about clicking an image says "remove".
 */
export function ImageUpload({ complex }: { complex: Complex }) {
  return (
    <div className="overflow-hidden rounded-xl border border-border-subtle bg-bg-elevated">
      <div className="relative">
        <Slot
          complexId={complex.id}
          type="cover"
          url={complex.cover_url}
          label={t.complex.coverLabel}
          className="aspect-[16/9] w-full"
          rounded={false}
          empty={
            <div className="flex flex-col items-center gap-1.5 text-text-tertiary">
              <ImageIcon className="size-7" />
              <span className="text-xs">{t.complex.noCover}</span>
            </div>
          }
        />
        {/* Overlapping the cover's lower edge, as on the storefront. */}
        <Slot
          complexId={complex.id}
          type="logo"
          url={complex.logo_url}
          label={t.complex.logoLabel}
          className="absolute -bottom-10 left-5 size-24 border-2 border-bg-elevated rounded-2xl"
          rounded
          empty={<Camera className="size-8 text-text-tertiary" />}
        />
      </div>

      <p className="px-5 pt-14 pb-5 text-micro leading-4 text-text-tertiary">
        {t.complex.imageSpecs}
        <br />
        {t.complex.imageFormats}
      </p>
    </div>
  );
}

/** The uploaded image itself, deferred and sized (see PREVIEW_SIZE). */
function SlotPreview({ url, label, type }: { url: string; label: string; type: 'logo' | 'cover' }) {
  return (
    <img
      src={url}
      alt={label}
      loading="lazy"
      decoding="async"
      width={PREVIEW_SIZE[type].width}
      height={PREVIEW_SIZE[type].height}
      className="h-full w-full object-cover"
    />
  );
}

/**
 * One image: the preview, the file input behind it, and its delete control.
 *
 * The whole slot lives in one component so the file input's ref never crosses
 * a prop boundary — a ref read from a parent's object inside a child is what
 * `react-hooks/refs` exists to stop.
 */
function Slot({
  complexId,
  type,
  url,
  label,
  className,
  rounded,
  empty,
}: {
  complexId: string;
  type: 'logo' | 'cover';
  url: string | null;
  label: string;
  className: string;
  rounded?: boolean;
  empty: React.ReactNode;
}) {
  const { fileRef, isLoading, handleFileChange, deleteMutation } = useImageSlot(complexId, type);

  return (
    <div className={cn('group/slot relative overflow-hidden', className)}>
      <input ref={fileRef} type="file" accept={ACCEPTED} className="hidden" onChange={handleFileChange} />
      <button
        type="button"
        aria-label={label}
        onClick={() => {
          if (!isLoading) fileRef.current?.click();
        }}
        className={cn(
          'focus-self relative flex h-full w-full items-center justify-center overflow-hidden',
          'bg-bg-base transition-[filter] hover:brightness-110',
          'focus-visible:ring-2 focus-visible:ring-primary-500',
          rounded && 'rounded-2xl',
        )}
      >
        {isLoading ? (
          <div className="size-5 animate-spin rounded-full border-2 border-primary-500/30 border-t-primary-500" />
        ) : url ? (
          <SlotPreview url={url} label={label} type={type} />
        ) : (
          empty
        )}
        {!isLoading && url && (
          <span className="absolute inset-0 flex items-center justify-center bg-black/0 opacity-0 transition-[background-color,opacity] group-hover/slot:bg-black/40 group-hover/slot:opacity-100">
            <Upload className="size-4 text-white" />
          </span>
        )}
      </button>
      {url && (
        <div className="absolute right-2 top-2 opacity-0 transition-opacity group-hover/slot:opacity-100 focus-within:opacity-100">
          <ImageDeleteConfirm
            disabled={isLoading}
            onConfirm={() => {
              deleteMutation.mutate({ type, currentUrl: url });
            }}
          />
        </div>
      )}
    </div>
  );
}
