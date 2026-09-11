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
            'group flex flex-col items-center justify-center rounded-2xl border border-dashed border-border-subtle p-4 transition-colors duration-150 sm:p-5',
            'hover:border-primary-500/30 hover:bg-bg-elevated/50',
            'press-scale min-h-[100px]',
          )}
        >
          <div className="flex size-10 items-center justify-center rounded-xl bg-bg-elevated">
            <Plus className="size-5 text-text-tertiary transition-colors group-hover:text-primary-500" />
          </div>
          <span className="mt-2 text-sm font-medium text-text-tertiary transition-colors group-hover:text-text-secondary">
            {t.complex.addComplex}
          </span>
        </button>
      )}
    </div>
  );
}
