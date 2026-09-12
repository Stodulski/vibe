import { Switch } from '@/shared/components/ui/switch';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

interface CardHeaderProps {
  court: CourtWithPrices;
  onToggleActive: () => void;
  isTogglePending: boolean;
}

export function CardHeader({ court, onToggleActive, isTogglePending }: CardHeaderProps) {
  return (
    <div className="flex items-start gap-3">
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <h3 className="text-text-primary truncate text-base font-semibold">{court.name}</h3>
          {/* The switch alone. The word beside it said what the knob's own
              position already says, twelve times over on a full grid — and the
              accessible name still spells the state out for anyone reading the
              card rather than looking at it. */}
          <Switch
            className="shrink-0"
            checked={court.is_active}
            onCheckedChange={onToggleActive}
            disabled={isTogglePending}
            aria-label={`${court.name}: ${court.is_active ? t.courts.active : t.courts.inactive}`}
          />
        </div>
        {/*
          The sport is named here and nowhere else. There used to be a badge
          beside the court name carrying an emoji for it — an emoji renders in
          whatever colour and drawing the viewer's platform ships, so it could
          not take the brand colour, and padel and tennis shared one 🎾 anyway.
          Nothing replaced it: lucide has no racquet and no basketball, and a
          badge whose icon does not name its sport is decoration. The word does
          the job.

          Sport and court type carry the brand green; the separator between
          them does not. They are what distinguishes one court from the next
          in a grid where every other line looks alike, and the green is what
          the eye finds first. 9.86:1 on the card ground, measured.

          No icons, though: the small glyph that used to sit beside
          "Descubierta" repeated the word next to it — texture, not
          information, on a card read a dozen at a time.
        */}
        <p className="text-text-tertiary mt-1 truncate text-xs">
          <span className="text-primary-400 font-medium">{t.courts.sportTypes[court.sport]}</span>
          {' · '}
          <span className="text-primary-400 font-medium">{t.courts.courtTypes[court.court_type]}</span>
        </p>
        {/* One line, always — including for a court that has none, which says
            so instead of collapsing. A card that grows or shrinks with the
            length of its description makes a grid of them ragged, and the row
            heights are what let the eye run down a column of names.

            The full text is on `title`, so the truncation costs nothing: the
            owner wrote it for a player, and here it is only a reminder of what
            is on the storefront. */}
        {!court.description ? (
          <p className="text-text-tertiary mt-1.5 truncate text-xs">{t.courts.noDescription}</p>
        ) : (
          <p className="text-text-secondary mt-1.5 truncate text-xs" title={court.description}>
            {court.description}
          </p>
        )}
      </div>
    </div>
  );
}
