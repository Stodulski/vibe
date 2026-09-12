import { MapPin } from 'lucide-react';
import { cn } from '@/shared/lib/utils';
import type { Prediction } from './types';

interface AddressPredictionListProps {
  predictions: Prediction[];
  activeIndex: number;
  onSelect: (prediction: Prediction) => void;
  onHover: (index: number) => void;
}

export function AddressPredictionList({ predictions, activeIndex, onSelect, onHover }: AddressPredictionListProps) {
  return (
    <ul
      id="address-listbox"
      role="listbox"
      className="border-border-subtle bg-bg-base absolute right-0 left-0 z-50 mt-1.5 max-h-[60vh] overflow-auto rounded-xl border p-1 shadow-lg sm:max-h-80"
    >
      {predictions.map((p, i) => (
        <li
          key={p.place_id}
          id={`address-option-${String(i)}`}
          role="option"
          aria-selected={i === activeIndex}
          className={cn(
            'text-nav flex cursor-pointer items-start gap-2 rounded-lg px-2.5 py-2.5 transition-colors sm:gap-2.5 sm:px-3 sm:py-3 sm:text-sm',
            i === activeIndex ? 'bg-primary-500/10 text-text-primary' : 'text-text-secondary hover:bg-bg-subtle',
          )}
          onMouseDown={() => {
            onSelect(p);
          }}
          onMouseEnter={() => {
            onHover(i);
          }}
        >
          <MapPin className="text-text-tertiary mt-0.5 size-3.5 shrink-0 sm:size-4" />
          <div className="min-w-0">
            <span className="text-text-primary font-medium">{p.structured_formatting.main_text}</span>
            <br className="sm:hidden" />
            <span className="text-text-tertiary sm:ml-1">{p.structured_formatting.secondary_text}</span>
          </div>
        </li>
      ))}
    </ul>
  );
}
