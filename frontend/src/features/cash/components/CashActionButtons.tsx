import { ArrowDownCircle, ArrowUpCircle, Lock } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface CashActionButtonsProps {
  onIncome: () => void;
  onExpense: () => void;
  onClose: () => void;
}

/** "Ingreso"/"Egreso"/"Cerrar caja" — big touch targets, reachable without horizontal scroll at 360px. */
export function CashActionButtons({ onIncome, onExpense, onClose }: CashActionButtonsProps) {
  return (
    <div className="grid grid-cols-2 gap-2 sm:flex sm:flex-wrap">
      <Button size="lg" className="h-12" onClick={onIncome}>
        <ArrowUpCircle className="size-4" aria-hidden="true" />
        {t.cash.incomeAction}
      </Button>
      <Button size="lg" variant="outline" className="h-12" onClick={onExpense}>
        <ArrowDownCircle className="size-4" aria-hidden="true" />
        {t.cash.expenseAction}
      </Button>
      <Button size="lg" variant="destructive" className="col-span-2 h-12 sm:col-span-1" onClick={onClose}>
        <Lock className="size-4" aria-hidden="true" />
        {t.cash.closeAction}
      </Button>
    </div>
  );
}
