import { Input } from '@/shared/components/ui/input';
import { ES_AR } from '@/shared/i18n/es_AR';
import { isSuperRow, type ScoreFormat, type ScoreRow } from '../../lib/score';

const t = ES_AR.tournaments.result;

interface SetRowsProps {
  rows: ScoreRow[];
  format: ScoreFormat;
  names: [string, string];
  onChange: (index: number, side: 'a' | 'b', value: string) => void;
}

function ScoreInput({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <Input
      type="number"
      inputMode="numeric"
      min={0}
      max={99}
      aria-label={label}
      value={value}
      className="score-text w-full text-center"
      onChange={(event) => {
        onChange(event.target.value);
      }}
    />
  );
}

/**
 * One row per set a match can take, as many as the category's format allows
 * (one, or three). The row that can be the match tiebreak counts points, not
 * games; a row left blank is a set not played.
 */
export function SetRows({ rows, format, names, onChange }: SetRowsProps) {
  return (
    <div className="space-y-3">
      {rows.map((row, index) => {
        const isSuper = isSuperRow(format, index);
        const setLabel = isSuper ? t.superTiebreakSet(index + 1) : t.set(index + 1);
        const label = isSuper ? t.points : t.games;
        return (
          <fieldset key={index} className="space-y-1.5">
            <legend className="text-text-secondary text-xs font-medium">{setLabel}</legend>
            {index > 0 && <p className="text-text-tertiary text-xs">{t.optionalSet}</p>}
            <div className="grid grid-cols-2 gap-3">
              <ScoreInput
                label={`${label(names[0])} (${setLabel})`}
                value={row.a}
                onChange={(value) => {
                  onChange(index, 'a', value);
                }}
              />
              <ScoreInput
                label={`${label(names[1])} (${setLabel})`}
                value={row.b}
                onChange={(value) => {
                  onChange(index, 'b', value);
                }}
              />
            </div>
          </fieldset>
        );
      })}
    </div>
  );
}
