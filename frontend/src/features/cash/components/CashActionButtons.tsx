import { ArrowDownCircle, ArrowUpCircle, Lock } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface CashActionButtonsProps {
  onIncome: () => void;
  onExpense: () => void;
  onClose: () => void;
}

/**
 * Ingreso is the one primary action, Egreso secondary. Cerrar caja happens
 * once a shift and cannot be undone, so it is tertiary and set apart, never
 * red: the red belongs to its confirmation dialog.
 */
export function CashActionButtons({ onIncome, onExpense, onClose }: CashActionButtonsProps) {
  return (
    <div className="grid grid-cols-2 gap-2 sm:flex sm:items-center">
      <Button size="lg" className="h-12" onClick={onIncome}>
        <ArrowUpCircle className="size-4" aria-hidden="true" />
        {t.cash.incomeAction}
      </Button>
      <Button size="lg" variant="outline" className="h-12" onClick={onExpense}>
        <ArrowDownCircle className="size-4" aria-hidden="true" />
        {t.cash.expenseAction}
      </Button>
      <Button size="lg" variant="ghost" className="col-span-2 h-12 sm:ml-auto" onClick={onClose}>
        <Lock className="size-4" aria-hidden="true" />
        {t.cash.closeAction}
      </Button>
    </div>
  );
}
