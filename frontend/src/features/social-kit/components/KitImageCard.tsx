import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { SocialKitFormat } from '@/shared/lib/publicPaths';
import { cn } from '@/shared/lib/utils';
import { KIT_CARD_CLASS, KIT_PREVIEW_ASPECT_CLASS } from './kitLayout';

const t = ES_AR;

interface KitImageCardProps {
  /** Title of the piece the image belongs to, used in its alt text and download label. */
  pieceTitle: string;
  format: SocialKitFormat;
  previewUrl: string;
  downloadUrl: string;
}

/** One image of a piece: its preview at the format's aspect ratio, and a link that downloads it. */
export function KitImageCard({ pieceTitle, format, previewUrl, downloadUrl }: KitImageCardProps) {
  const formatLabel = t.navigation.socialKitFormats[format];
  const imageName = `${formatLabel}: ${pieceTitle}`;

  return (
    <article className={KIT_CARD_CLASS}>
      <h3 className="text-text-primary text-sm leading-5 font-semibold">{formatLabel}</h3>
      <div className={cn('bg-bg-base w-full overflow-hidden rounded-lg', KIT_PREVIEW_ASPECT_CLASS[format])}>
        <img src={previewUrl} alt={imageName} loading="lazy" className="size-full object-contain" />
      </div>
      <Button asChild className="w-full rounded-full">
        <a href={downloadUrl}>
          {t.navigation.socialKitDownload}
          {/* Four identical "Descargar" links on one page: the hidden suffix tells them apart. */}
          <span className="sr-only"> {imageName}</span>
        </a>
      </Button>
    </article>
  );
}
