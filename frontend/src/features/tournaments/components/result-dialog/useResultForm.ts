import { useState } from 'react';
import { useSetResult } from '../../hooks/useTournamentChanges';
import { resultErrorMessage } from '../../lib/tournamentErrors';
import { parseScore, scoreFormatOf, scoreRowCount, type ScoreRow } from '../../lib/score';
import type { TournamentCategory, TournamentMatch } from '@/shared/types/api.types';

export type ResultMode = 'sets' | 'walkover';

/** The rows to start from: a loaded result's sets (to correct it), then blank rows up to the most sets a match takes. */
function initialRows(match: TournamentMatch, rowCount: number): ScoreRow[] {
  return Array.from({ length: rowCount }, (_, i) => {
    const set = match.sets[i];
    return { a: set ? String(set.a) : '', b: set ? String(set.b) : '' };
  });
}

interface Options {
  complexId: string;
  tournamentId: string;
  category: TournamentCategory;
  match: TournamentMatch;
  onDone: () => void;
}

/**
 * The result form: sets typed as text (a blank row is a set not played) or a
 * walkover. A bad score is told here before sending; what the server refuses
 * (the next match already has a result) comes back as `serverError`.
 */
export function useResultForm({ complexId, tournamentId, category, match, onDone }: Options) {
  const format = scoreFormatOf(category);
  const setResult = useSetResult(complexId, tournamentId);
  const [mode, setMode] = useState<ResultMode>(match.walkover_loser === 0 ? 'sets' : 'walkover');
  const [rows, setRows] = useState(() => initialRows(match, scoreRowCount(format)));
  const [loser, setLoser] = useState<1 | 2 | null>(match.walkover_loser === 0 ? null : match.walkover_loser);
  const [scoreError, setScoreError] = useState<string | null>(null);

  const setRow = (index: number, side: 'a' | 'b', value: string) => {
    setRows((prev) => prev.map((row, i) => (i === index ? { ...row, [side]: value } : row)));
    setScoreError(null);
  };

  const submit = () => {
    const send = (result: Parameters<typeof setResult.mutate>[0]['result']) => {
      setResult.mutate({ matchId: match.id, result }, { onSuccess: onDone });
    };
    if (mode === 'walkover') {
      if (loser !== null) send({ walkover_loser: loser });
      return;
    }
    const parsed = parseScore(rows, format);
    if (parsed.ok) send({ sets: parsed.sets });
    else setScoreError(parsed.error);
  };

  return {
    format,
    mode,
    setMode,
    rows,
    setRow,
    loser,
    setLoser,
    submit,
    isPending: setResult.isPending,
    canSubmit: mode === 'sets' || loser !== null,
    error: scoreError ?? (setResult.error ? resultErrorMessage(setResult.error) : null),
  };
}
