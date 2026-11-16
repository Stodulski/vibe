import { Button } from '@/shared/components/ui/button';

interface SidePickerProps {
  names: [string, string];
  value: 1 | 2 | null;
  onChange: (side: 1 | 2) => void;
  question: string;
  hint: string;
}

/** Which of the two pairs: one button per pair name, the chosen one pressed. */
export function SidePicker({ names, value, onChange, question, hint }: SidePickerProps) {
  return (
    <div className="space-y-2">
      <p className="text-text-primary text-sm font-medium">{question}</p>
      <div className="grid grid-cols-1 gap-2">
        {([1, 2] as const).map((side) => (
          <Button
            key={side}
            type="button"
            variant={value === side ? 'default' : 'outline'}
            aria-pressed={value === side}
            className="h-auto min-h-10 justify-start rounded-xl py-2 text-left break-words whitespace-normal"
            onClick={() => {
              onChange(side);
            }}
          >
            {names[side - 1]}
          </Button>
        ))}
      </div>
      <p className="text-text-tertiary text-xs">{hint}</p>
    </div>
  );
}
