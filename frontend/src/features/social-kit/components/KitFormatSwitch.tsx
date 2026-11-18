import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { SOCIAL_KIT_FORMATS, type SocialKitFormat } from '@/shared/lib/publicPaths';
import { cn } from '@/shared/lib/utils';
import { KIT_SWITCH_CLASS, KIT_SWITCH_SEGMENT_CLASS } from './kitLayout';

const t = ES_AR;

interface KitFormatSwitchProps {
  active: SocialKitFormat;
  onChange: (format: SocialKitFormat) => void;
}

/** Historia or Post: which format the page previews and downloads. */
export function KitFormatSwitch({ active, onChange }: KitFormatSwitchProps) {
  return (
    <div role="group" aria-label={t.navigation.socialKitFormatGroup} className={KIT_SWITCH_CLASS}>
      {SOCIAL_KIT_FORMATS.map((format) => {
        const selected = active === format;
        return (
          <Button
            key={format}
            type="button"
            variant={selected ? 'default' : 'outline'}
            aria-pressed={selected}
            className={cn(KIT_SWITCH_SEGMENT_CLASS, selected && 'border-transparent')}
            onClick={() => {
              onChange(format);
            }}
          >
            {t.navigation.socialKitFormats[format]}
          </Button>
        );
      })}
    </div>
  );
}
