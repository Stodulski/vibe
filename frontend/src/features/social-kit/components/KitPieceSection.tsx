import { ES_AR } from '@/shared/i18n/es_AR';
import { SOCIAL_KIT_FORMATS, socialKitImageUrl, type SocialKitPiece } from '@/shared/lib/publicPaths';
import { KitImageCard } from './KitImageCard';
import { KIT_PIECE_GRID_CLASS } from './kitLayout';

const t = ES_AR;

interface KitPieceSectionProps {
  piece: SocialKitPiece;
  /** Slug of the owner's complex, which the landing renders the images for. */
  slug: string;
}

/** One piece of the kit: its title, then its story and post cards. */
export function KitPieceSection({ piece, slug }: KitPieceSectionProps) {
  const title = t.navigation.socialKitPieces[piece];
  const headingId = `kit-${piece}`;

  return (
    <section aria-labelledby={headingId} className="space-y-4">
      <h2 id={headingId} className="font-display text-text-primary text-lg leading-7 font-semibold">
        {title}
      </h2>
      <div className={KIT_PIECE_GRID_CLASS}>
        {SOCIAL_KIT_FORMATS.map((format) => (
          <KitImageCard
            key={format}
            pieceTitle={title}
            format={format}
            previewUrl={socialKitImageUrl(slug, piece, format)}
            downloadUrl={socialKitImageUrl(slug, piece, format, { download: true })}
          />
        ))}
      </div>
    </section>
  );
}
