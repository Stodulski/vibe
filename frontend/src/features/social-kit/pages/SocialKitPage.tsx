import { useState } from 'react';
import { PageHeader } from '@/shared/components/common/PageHeader';
import { useSelectedComplex } from '@/features/complex';
import { usePageTitle } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SOCIAL_KIT_PIECES, socialKitImageUrl, type SocialKitFormat } from '@/shared/lib/publicPaths';
import { KitFormatSwitch } from '../components/KitFormatSwitch';
import { KitImageCard } from '../components/KitImageCard';
import { KIT_GRID_CLASS } from '../components/kitLayout';

const t = ES_AR;

/**
 * Owner page with the complex's social kit. The owner picks a format, Historia or Post, and
 * the two pieces show side by side in it: same shape, so the grid stays even.
 */
export default function SocialKitPage() {
  usePageTitle(t.navigation.socialKit);
  const { complex } = useSelectedComplex();
  const [format, setFormat] = useState<SocialKitFormat>('historia');

  // No slug until the selected complex has loaded, so there is nothing to link to yet (as on the dashboard).
  if (!complex?.slug) return null;
  const slug = complex.slug;

  return (
    <div className="space-y-6">
      <PageHeader title={t.navigation.socialKit} />
      <p className="text-text-secondary max-w-xl text-sm leading-relaxed">{t.navigation.socialKitIntro}</p>
      <div className="space-y-2">
        <KitFormatSwitch active={format} onChange={setFormat} />
        <p className="text-text-tertiary text-xs leading-5">{t.navigation.socialKitFormatHints[format]}</p>
      </div>
      <div className={KIT_GRID_CLASS}>
        {SOCIAL_KIT_PIECES.map((piece) => (
          <KitImageCard
            key={piece}
            pieceTitle={t.navigation.socialKitPieces[piece]}
            format={format}
            previewUrl={socialKitImageUrl(slug, piece, format)}
            downloadUrl={socialKitImageUrl(slug, piece, format, { download: true })}
          />
        ))}
      </div>
    </div>
  );
}
