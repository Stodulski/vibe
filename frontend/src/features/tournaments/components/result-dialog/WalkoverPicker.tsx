import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR.tournaments.result;

interface WalkoverPickerProps {
  names: [string, string];
  loser: 1 | 2 | null;
  onChange: (loser: 1 | 2) => void;
}

/** Which pair did not show up: the other one wins by the maximum score. */
export function WalkoverPicker({ names, loser, onChange }: WalkoverPickerProps) {
  return (
    <div className="space-y-2">
      <p className="text-text-primary text-sm font-medium">{t.walkoverQuestion}</p>
      <div className="grid grid-cols-1 gap-2">
        {([1, 2] as const).map((side) => (
          <Button
            key={side}
            type="button"
            variant={loser === side ? 'default' : 'outline'}
            aria-pressed={loser === side}
            className="h-auto min-h-10 justify-start rounded-xl py-2 text-left break-words whitespace-normal"
            onClick={() => {
              onChange(side);
            }}
          >
            {names[side - 1]}
          </Button>
        ))}
      </div>
      <p className="text-text-tertiary text-xs">{t.walkoverHint}</p>
    </div>
  );
}
