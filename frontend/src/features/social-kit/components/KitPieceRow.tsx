import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SOCIAL_KIT_FORMATS, socialKitImageUrl, type SocialKitPiece } from '@/shared/lib/publicPaths';
import { KIT_ACTIONS_CLASS, KIT_ROW_CLASS, KIT_THUMB_CLASS } from './kitLayout';

const t = ES_AR;

interface KitPieceRowProps {
  piece: SocialKitPiece;
  /** Slug of the owner's complex, which the landing renders the images for. */
  slug: string;
}

/** One piece of the kit as a card: the post as a preview, its title, and a download button per format. */
export function KitPieceRow({ piece, slug }: KitPieceRowProps) {
  const title = t.navigation.socialKitPieces[piece];
  const headingId = `kit-${piece}`;

  return (
    <article aria-labelledby={headingId} className={KIT_ROW_CLASS}>
      <div className={KIT_THUMB_CLASS}>
        <img
          src={socialKitImageUrl(slug, piece, 'post')}
          alt={`${t.navigation.socialKitFormats.post}: ${title}`}
          loading="lazy"
          className="bg-bg-base size-full object-cover"
        />
      </div>
      <div className="flex flex-col gap-3 px-1 pb-1">
        <h2 id={headingId} className="font-display text-text-primary text-lg leading-7 font-semibold">
          {title}
        </h2>
        <div className={KIT_ACTIONS_CLASS}>
          {SOCIAL_KIT_FORMATS.map((format) => (
            <Button
              key={format}
              asChild
              variant={format === 'historia' ? 'default' : 'outline'}
              className="w-full rounded-full"
            >
              <a
                href={socialKitImageUrl(slug, piece, format, { download: true })}
                // The same two labels on every row: the accessible name adds the piece.
                aria-label={`${t.navigation.socialKitDownloads[format]}: ${title}`}
              >
                {t.navigation.socialKitDownloads[format]}
              </a>
            </Button>
          ))}
        </div>
      </div>
    </article>
  );
}
