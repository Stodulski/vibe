import { BOOKING_MESSAGES } from '../messages';
import type { DeclaredGender, Modality, PublicOpenPlayEvent, PublicTournamentCategory } from '../api/types';
import type { Problem } from './ApiError';
import { isModality, levelLabel, levelRangeLabel, modalityLabel } from './traitLabels';

/**
 * The rules a player's declaration (gender and level) is checked against, in
 * the browser before anything is sent, and by the server, which refuses with
 * the same four codes. Each rule answers with the field it is about, so a form
 * can show the message beside it.
 */

const t = BOOKING_MESSAGES.declaration;

/** What one player declared. `undefined` is what the form has not been given yet. */
export interface Declared {
  gender?: DeclaredGender | undefined;
  /** 1ra (1) to 9na (9). */
  level?: number | undefined;
}

/** A field a rule is about. `partner.` is the partner's own. */
export type DeclaredField = 'gender' | 'level' | 'partner.gender' | 'partner.level';

export interface DeclarationIssue {
  field: DeclaredField;
  message: string;
}

type PairCategory = Pick<PublicTournamentCategory, 'modality' | 'level' | 'suma'>;
type EventTraits = Pick<PublicOpenPlayEvent, 'modality' | 'level_min' | 'level_max'>;

/** Men's both men, women's both women, mixed one of each. */
function pairFits(modality: Modality, payer: DeclaredGender, partner: DeclaredGender): boolean {
  if (modality === 'men') return payer === 'men' && partner === 'men';
  if (modality === 'women') return payer === 'women' && partner === 'women';
  return payer !== partner;
}

function pairModalityMessage(modality: Modality): string {
  if (modality === 'men') return t.pairMenOnly;
  if (modality === 'women') return t.pairWomenOnly;
  return t.pairMixed;
}

/** A gender the modality takes. Mixed takes both, so it has no copy. */
function eventGenderMessage(modality: Modality): string | undefined {
  if (modality === 'men') return t.openPlayMenOnly;
  if (modality === 'women') return t.openPlayWomenOnly;
  return undefined;
}

function eventGenderFits(modality: Modality, gender: DeclaredGender): boolean {
  return modality === 'mixed' || modality === gender;
}

function eventRangeMessage(min: number, max: number): string {
  return min === max ? t.openPlaySolo(levelLabel(min)) : t.openPlayRange(levelRangeLabel(min, max));
}

function levelTooStrongMessage(level: number): string {
  return t.levelTooStrong(levelLabel(level));
}

function sumaTooLowMessage(suma: number): string {
  return t.sumaTooLow(suma);
}

/**
 * Why a pair does not fit its category, field by field. A category that
 * predates the traits takes any pair. Missing declarations are not answered
 * here: the form requires them before it asks this.
 */
export function pairIssues(category: PairCategory, payer: Declared, partner: Declared): DeclarationIssue[] {
  if (!isModality(category.modality)) return [];
  const issues: DeclarationIssue[] = [];
  if (payer.gender && partner.gender && !pairFits(category.modality, payer.gender, partner.gender)) {
    issues.push({ field: 'partner.gender', message: pairModalityMessage(category.modality) });
  }
  if (typeof category.level === 'number') {
    if (typeof payer.level === 'number' && payer.level < category.level) {
      issues.push({ field: 'level', message: levelTooStrongMessage(category.level) });
    }
    if (typeof partner.level === 'number' && partner.level < category.level) {
      issues.push({ field: 'partner.level', message: levelTooStrongMessage(category.level) });
    }
  }
  if (
    typeof category.suma === 'number' &&
    typeof payer.level === 'number' &&
    typeof partner.level === 'number' &&
    payer.level + partner.level < category.suma
  ) {
    issues.push({ field: 'partner.level', message: sumaTooLowMessage(category.suma) });
  }
  return issues;
}

/** Why one person does not fit an open-play event: the gender its modality takes, and its range of levels. */
export function openPlayIssues(event: EventTraits, player: Declared): DeclarationIssue[] {
  const issues: DeclarationIssue[] = [];
  if (isModality(event.modality) && player.gender && !eventGenderFits(event.modality, player.gender)) {
    const message = eventGenderMessage(event.modality);
    if (message) issues.push({ field: 'gender', message });
  }
  const { level_min: min, level_max: max } = event;
  if (typeof min === 'number' && typeof max === 'number' && typeof player.level === 'number') {
    if (player.level < min || player.level > max) {
      issues.push({ field: 'level', message: eventRangeMessage(min, max) });
    }
  }
  return issues;
}

/** The line under a pair sign-up's heading: "Caballeros · 7ma o menor", or the Suma it needs. `null` when there is none. */
export function pairRequirement(category: PairCategory): string | null {
  if (!isModality(category.modality)) return null;
  const who = modalityLabel(category.modality);
  if (typeof category.level === 'number') return `${who} · ${t.levelOrWeaker(levelLabel(category.level))}`;
  if (typeof category.suma === 'number') return `${who} · ${t.sumaRequirement(category.suma)}`;
  return who;
}

/** The line under an open-play sign-up's heading: "Caballeros · de 6ta a 8va", or "Damas · solo 4ta". `null` when there is none. */
export function openPlayRequirement(event: EventTraits): string | null {
  const parts: string[] = [];
  if (isModality(event.modality)) parts.push(modalityLabel(event.modality));
  const { level_min: min, level_max: max } = event;
  if (typeof min === 'number' && typeof max === 'number') {
    const range = levelRangeLabel(min, max);
    parts.push(min === max ? range : `de ${range}`);
  }
  return parts.length > 0 ? parts.join(' · ') : null;
}

/** The form's strings as a declaration: an empty gender or level is left out. */
export function toDeclared(gender: string, level: string): Declared {
  return {
    gender: gender === 'men' || gender === 'women' ? gender : undefined,
    level: /^[1-9]$/.test(level) ? Number(level) : undefined,
  };
}

/** What a declaration is checked against: the category of a pair, or the event of one person. */
export type DeclarationSubject = { kind: 'pair'; category: PairCategory } | { kind: 'event'; event: EventTraits };

function refusalFor(code: string, subject: DeclarationSubject): string | undefined {
  if (subject.kind === 'pair') {
    const { category } = subject;
    if (code === 'modality_mismatch' && isModality(category.modality)) return pairModalityMessage(category.modality);
    if (code === 'level_too_strong' && typeof category.level === 'number') return levelTooStrongMessage(category.level);
    if (code === 'suma_too_low' && typeof category.suma === 'number') return sumaTooLowMessage(category.suma);
    return undefined;
  }
  const { event } = subject;
  if (code === 'modality_mismatch' && isModality(event.modality)) return eventGenderMessage(event.modality);
  if (code === 'level_out_of_range' && typeof event.level_min === 'number' && typeof event.level_max === 'number') {
    return eventRangeMessage(event.level_min, event.level_max);
  }
  return undefined;
}

/**
 * The copy for a refused sign-up whose declaration did not fit, read from the
 * problem's field errors, or `undefined` when the refusal names none of them.
 */
export function declarationRefusalCopy(
  problem: Pick<Problem, 'status' | 'errors'> | undefined,
  subject: DeclarationSubject,
): string | undefined {
  if (problem?.status !== 422) return undefined;
  for (const error of problem.errors) {
    const copy = refusalFor(error.message, subject);
    if (copy) return copy;
  }
  return undefined;
}
