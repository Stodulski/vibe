import { useId } from 'react';
import { Label } from '@/shared/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/shared/components/ui/select';
import { TOURNAMENT_SWITCH_FIELD_CLASS } from '../lib/tournamentLayout';

interface LabeledSelectProps<T extends string> {
  label: string;
  items: { value: T; label: string }[];
  active: T;
  onChange: (value: T) => void;
}

/** The app's Select with a visible label above it: the owner's category and section pickers. */
export function LabeledSelect<T extends string>({ label, items, active, onChange }: LabeledSelectProps<T>) {
  const id = useId();
  return (
    <div className={TOURNAMENT_SWITCH_FIELD_CLASS}>
      <Label htmlFor={id}>{label}</Label>
      <Select
        value={active}
        onValueChange={(value) => {
          const next = items.find((item) => item.value === value);
          if (next) onChange(next.value);
        }}
      >
        <SelectTrigger id={id} className="w-full min-w-0 [&>span]:truncate">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {items.map((item) => (
            <SelectItem key={item.value} value={item.value}>
              {item.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
