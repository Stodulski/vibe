import { ES_AR } from '@/shared/i18n/es_AR';
import { Button } from '@/shared/components/ui/button';

const t = ES_AR;

interface QuickBookIdentityConfirmProps {
  onConfirm: () => void;
  onNotMe: () => void;
}

/**
 * "Sí, soy yo" / "No, soy otra persona" — two equally prominent choices
 * `QuickBookView` shows before anything saved can be submitted. See its own
 * comment for why this exists.
 */
export function QuickBookIdentityConfirm({ onConfirm, onNotMe }: QuickBookIdentityConfirmProps) {
  return (
    <div className="border-border-subtle flex flex-col gap-2 border-t pt-3">
      <p className="text-text-secondary text-xs font-medium">{t.publicBooking.quickBookConfirmPrompt}</p>
      <div className="flex gap-2">
        <Button type="button" size="sm" className="flex-1" onClick={onConfirm}>
          {t.publicBooking.quickBookConfirmYes}
        </Button>
        <Button type="button" variant="outline" size="sm" className="flex-1" onClick={onNotMe}>
          {t.publicBooking.quickBookConfirmNo}
        </Button>
      </div>
    </div>
  );
}
