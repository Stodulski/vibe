import { Building2, Plus } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface NoComplexesStateProps {
  onAddComplex: () => void;
}

export function NoComplexesState({ onAddComplex }: NoComplexesStateProps) {
  return (
    <div className="border-border-subtle bg-bg-elevated flex flex-col items-center rounded-2xl border p-6 sm:p-12">
      <div className="bg-primary-500/10 flex size-14 items-center justify-center rounded-2xl">
        <Building2 className="text-primary-500 size-7" />
      </div>
      <p className="text-text-secondary mt-4 text-center text-sm font-medium">{t.complex.noComplexes}</p>
      <Button onClick={onAddComplex} className="mt-6">
        <Plus className="size-4" />
        {t.complex.addComplex}
      </Button>
    </div>
  );
}
