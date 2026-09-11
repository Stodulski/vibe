import { Badge } from '@/shared/components/ui/badge';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface ActiveStatusBadgeProps {
  isActive: boolean;
  className?: string;
}

/**
 * Shared active/inactive status badge. "Inactive" renders as the muted
 * `secondary` variant, not `destructive` — system colours are reserved for
 * actual error/status states, not a neutral "off" state.
 */
export function ActiveStatusBadge({ isActive, className }: ActiveStatusBadgeProps) {
  return (
    <Badge variant={isActive ? 'default' : 'secondary'} className={className}>
      {isActive ? t.admin.users.active : t.admin.users.inactive}
    </Badge>
  );
}
