import { useState } from 'react';
import { useSetResult } from '../../hooks/useTournamentChanges';
import { ES_AR } from '@/shared/i18n/es_AR';
import { resultErrorMessage } from '../../lib/tournamentErrors';
import { parseScore, scoreFormatOf, scoreRowCount, type ScoreRow } from '../../lib/score';
import type { TournamentCategory, TournamentMatch } from '@/shared/types/api.types';

export type ResultMode = 'winner' | 'sets' | 'walkover';

/** The mode a match opens in: its walkover, its score, or (new or winner-only) just who won. */
function initialMode(match: TournamentMatch): ResultMode {
  if (match.walkover_loser !== 0) return 'walkover';
  return match.sets.length > 0 ? 'sets' : 'winner';
}

/** The side of a loaded result's winner, if any. */
function winnerSide(match: TournamentMatch): 1 | 2 | null {
  if (!match.winner_pair_id) return null;
  if (match.winner_pair_id === match.pair1_id) return 1;
  return match.winner_pair_id === match.pair2_id ? 2 : null;
}

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
 * The result form: only who won (the default), the sets typed as text (a blank
 * row is a set not played) or a walkover. A bad score is told here before sending; what the server refuses
 * (the next match already has a result) comes back as `serverError`.
 */
export function useResultForm({ complexId, tournamentId, category, match, onDone }: Options) {
  const format = scoreFormatOf(category);
  const setResult = useSetResult(complexId, tournamentId);
  const [mode, setMode] = useState<ResultMode>(() => initialMode(match));
  const [winner, setWinner] = useState<1 | 2 | null>(() => winnerSide(match));
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
    // The save button is never disabled: a missing pick is told here, on pressing it.
    if (mode === 'winner') {
      if (winner !== null) send({ winner });
      else setScoreError(ES_AR.tournaments.result.errors.pickWinner);
      return;
    }
    if (mode === 'walkover') {
      if (loser !== null) send({ walkover_loser: loser });
      else setScoreError(ES_AR.tournaments.result.errors.pickAbsent);
      return;
    }
    const parsed = parseScore(rows, format);
    if (parsed.ok) send({ sets: parsed.sets });
    else setScoreError(parsed.error);
  };

  return {
    format,
    mode,
    setMode: (next: ResultMode) => {
      setMode(next);
      setScoreError(null);
    },
    rows,
    setRow,
    winner,
    setWinner: (side: 1 | 2) => {
      setWinner(side);
      setScoreError(null);
    },
    loser,
    setLoser: (side: 1 | 2) => {
      setLoser(side);
      setScoreError(null);
    },
    submit,
    isPending: setResult.isPending,
    error: scoreError ?? (setResult.error ? resultErrorMessage(setResult.error) : null),
  };
}
