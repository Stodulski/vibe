import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/shared/lib/utils';

export type StatusHeroTone = 'success' | 'warning' | 'error' | 'info' | 'neutral';
export type StatusHeroSize = 'default' | 'large';

const STATUS_HERO_TONES: Record<StatusHeroTone, string> = {
  success: 'bg-success-bg ring-success-border/20 text-success-icon',
  warning: 'bg-warning-bg ring-warning-border/20 text-warning-text',
  error: 'bg-error-bg ring-error-border/20 text-error-text',
  info: 'bg-info-bg ring-info-border/20 text-info-text',
  neutral: 'bg-bg-elevated ring-border/20 text-text-secondary',
};

const BADGE_SIZE: Record<StatusHeroSize, string> = {
  default: 'size-16 sm:size-20',
  large: 'size-20 sm:size-24',
};

const ICON_SIZE: Record<StatusHeroSize, string> = {
  default: 'size-8 sm:size-10',
  large: 'size-10 sm:size-14',
};

// `large` (the two terminal money-outcome screens, SuccessState and
// CancelledResultState) gets one extra step of breathing room between the
// title and the summary content below it than `default`'s polling/error
// screens do.
const STACK_GAP: Record<StatusHeroSize, string> = {
  default: 'gap-4 sm:gap-5',
  large: 'gap-5 sm:gap-6',
};

interface StatusHeroProps {
  icon: LucideIcon;
  tone: StatusHeroTone;
  /** `large` is only used by the terminal happy-path screen (SuccessState). */
  size?: StatusHeroSize;
  /** Plain string in every current caller; accepts a node so a future caller can compose its own styling instead of reaching for `titleClassName`. */
  title: ReactNode;
  /** Omit when the site has no plain description text (e.g. SuccessState, whose body is a set of cards passed as children). */
  description?: ReactNode;
  /** Optional action button/content rendered below the description. */
  children?: ReactNode;
  /**
   * False for the two static resolveLink-failure screens (LinkExpiredState,
   * LinkNotFoundState): they render before any status polling starts, so
   * they skip the fade/spring-in entrance and the badge's responsive growth,
   * and use a narrower, more tightly padded wrapper. Defaults to true, the
   * animated variant used by the booking-status polling flow.
   */
  animated?: boolean;
  /**
   * Escape hatch for CancelledState's larger (text-2xl) title — every other
   * default-size screen uses text-xl. Kept for that existing caller; a new
   * one should pass a styled node as `title` instead of adding another class.
   */
  titleClassName?: string;
  /**
   * Escape hatch for TimeoutState/PaymentUnderReviewState's `max-w-sm`
   * description. Kept for those existing callers; a new one should pass a
   * styled node as `description` instead of adding another class.
   */
  descriptionClassName?: string;
  /**
   * Every status screen centers its badge and title over the content; `start`
   * is kept for a caller that needs the left-aligned column.
   */
  align?: 'start' | 'center';
}

export function StatusHero({
  icon: Icon,
  tone,
  size = 'default',
  title,
  description,
  children,
  animated = true,
  titleClassName,
  descriptionClassName,
  align = 'center',
}: StatusHeroProps) {
  const badge = (
    <div
      className={cn(
        'flex items-center justify-center rounded-full ring-4',
        animated ? BADGE_SIZE[size] : 'size-16',
        STATUS_HERO_TONES[tone],
      )}
    >
      <Icon className={cn(animated ? ICON_SIZE[size] : 'size-8')} />
    </div>
  );

  return (
    <div
      className={cn(
        'mx-auto flex w-full max-w-md flex-col',
        align === 'center' ? 'items-center text-center' : 'items-start text-left',
        animated ? cn(STACK_GAP[size], 'animate-fade-in px-2 py-10 sm:px-0 sm:py-16') : 'gap-4 px-4 py-16',
      )}
    >
      {animated ? <div className="animate-spring-scale">{badge}</div> : badge}
      <h2 className={cn('text-text-primary font-bold', titleClassName ?? (size === 'large' ? 'text-2xl' : 'text-xl'))}>
        {title}
      </h2>
      {description && <p className={cn('text-text-secondary text-sm', descriptionClassName)}>{description}</p>}
      {children && (
        <div className={cn('mt-4 flex w-full flex-col', align === 'center' ? 'items-center' : 'items-start')}>
          {children}
        </div>
      )}
    </div>
  );
}
