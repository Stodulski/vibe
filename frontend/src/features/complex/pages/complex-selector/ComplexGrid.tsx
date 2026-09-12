import { Plus } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import type { Complex } from '@/shared/types/api.types';
import { ComplexCard } from './ComplexCard';

const t = ES_AR;

interface ComplexGridProps {
  complexes: Complex[];
  onSelect: (complex: Complex) => void;
  onAddComplex: () => void;
}

export function ComplexGrid({ complexes, onSelect, onAddComplex }: ComplexGridProps) {
  return (
    <div className="grid grid-cols-1 gap-3 sm:gap-4">
      {complexes.map((complex) => (
        <ComplexCard key={complex.id} complex={complex} onSelect={onSelect} />
      ))}

      {/* Add complex card (max 4) */}
      {complexes.length < 4 && (
        <button
          onClick={onAddComplex}
          className={cn(
            'group border-border-subtle flex flex-col items-center justify-center rounded-2xl border border-dashed p-4 transition-colors duration-150 sm:p-5',
            'hover:border-primary-500/30 hover:bg-bg-elevated/50',
            'press-scale min-h-[100px]',
          )}
        >
          <div className="bg-bg-elevated flex size-10 items-center justify-center rounded-xl">
            <Plus className="text-text-tertiary group-hover:text-primary-500 size-5 transition-colors" />
          </div>
          <span className="text-text-tertiary group-hover:text-text-secondary mt-2 text-sm font-medium transition-colors">
            {t.complex.addComplex}
          </span>
        </button>
      )}
    </div>
  );
}
