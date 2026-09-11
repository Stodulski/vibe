import { useNavigate } from 'react-router-dom';
import { ChevronRight } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';

const t = ES_AR;

interface ViewAllBookingsButtonProps {
  /** Only shown at/above `md:` — the mobile counterpart stays hidden there instead. */
  desktopOnly?: boolean;
}

export function ViewAllBookingsButton({ desktopOnly }: ViewAllBookingsButtonProps) {
  const navigate = useNavigate();

  return (
    <Button
      variant="ghost"
      size="sm"
      className={cn(
        'mt-3 w-full gap-1.5 text-sm text-primary-400 hover:text-primary-300',
        desktopOnly ? 'hidden md:flex' : 'md:hidden',
      )}
      onClick={() => {
        void navigate('/bookings');
      }}
    >
      {t.dashboard.viewAll}
      <ChevronRight className="size-3.5" aria-hidden="true" />
    </Button>
  );
}
