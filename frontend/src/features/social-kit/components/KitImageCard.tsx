import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { SocialKitFormat } from '@/shared/lib/publicPaths';
import { cn } from '@/shared/lib/utils';
import { KIT_CARD_CLASS, KIT_PREVIEW_ASPECT_CLASS } from './kitLayout';

const t = ES_AR;

interface KitImageCardProps {
  /** Title of the piece, shown under the preview and used in its alt text and download label. */
  pieceTitle: string;
  format: SocialKitFormat;
  previewUrl: string;
  downloadUrl: string;
}

/** One piece in the chosen format: its preview at the format's aspect ratio, its title and its download link. */
export function KitImageCard({ pieceTitle, format, previewUrl, downloadUrl }: KitImageCardProps) {
  const imageName = `${t.navigation.socialKitFormats[format]}: ${pieceTitle}`;

  return (
    <article className={KIT_CARD_CLASS}>
      <div className={cn('bg-bg-base w-full overflow-hidden rounded-xl', KIT_PREVIEW_ASPECT_CLASS[format])}>
        <img src={previewUrl} alt={imageName} loading="lazy" className="size-full object-cover" />
      </div>
      <h2 className="text-text-primary truncate px-1 text-sm leading-5 font-semibold sm:text-base">{pieceTitle}</h2>
      <Button asChild className="w-full rounded-full">
        <a href={downloadUrl}>
          {t.navigation.socialKitDownload}
          {/* Two identical "Descargar" links on one page: the hidden suffix tells them apart. */}
          <span className="sr-only"> {imageName}</span>
        </a>
      </Button>
    </article>
  );
}
