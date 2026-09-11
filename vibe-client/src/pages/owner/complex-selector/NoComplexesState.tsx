import { Building2, Plus } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface NoComplexesStateProps {
  onAddComplex: () => void;
}

export function NoComplexesState({ onAddComplex }: NoComplexesStateProps) {
  return (
    <div className="flex flex-col items-center rounded-2xl border border-border-subtle bg-bg-elevated p-6 sm:p-12">
      <div className="flex size-14 items-center justify-center rounded-2xl bg-primary-500/10">
        <Building2 className="size-7 text-primary-500" />
      </div>
      <p className="mt-4 text-center text-sm font-medium text-text-secondary">{t.complex.noComplexes}</p>
      <Button onClick={onAddComplex} className="mt-6">
        <Plus className="size-4" />
        {t.complex.addComplex}
      </Button>
    </div>
  );
}
