import { describe, it, expect, beforeEach } from 'vitest';
import { safeSessionStorage } from '@vibe/ui';
import {
  bookingResultKey,
  clearBookingResult,
  clearConfirmDraft,
  confirmDraftKey,
  readBookingResult,
  readConfirmDraft,
  saveBookingResult,
  saveConfirmDraft,
  saveHoldExpiredNotice,
  hasHoldExpiredNotice,
  clearHoldExpiredNotice,
  type ConfirmDraft,
} from './handoff';

const DRAFT: ConfirmDraft = {
  complexId: 'c1',
  complexName: 'Club Norte',
  complexPhone: '1155550000',
  courtId: 'court-1',
  courtName: 'Cancha 1',
  sport: 'padel',
  date: '2026-03-18',
  startTime: '10:00',
  endTime: '11:30',
  durationMinutes: 90,
  price: 1000,
  depositPercentage: 30,
  cancellationHours: 24,
  holdToken: 'hold-1',
  holdExpiresAt: '2026-03-18T13:15:00.000Z',
};

beforeEach(() => {
  window.sessionStorage.clear();
});

describe('confirm draft', () => {
  it('returns the draft that was saved for the same slug', () => {
    saveConfirmDraft('club-norte', DRAFT);

    expect(readConfirmDraft('club-norte')).toEqual(DRAFT);
  });

  it('keeps one draft per slug', () => {
    saveConfirmDraft('club-norte', DRAFT);

    expect(readConfirmDraft('otro-club')).toBeNull();
  });

  it('returns null when nothing was saved', () => {
    expect(readConfirmDraft('club-norte')).toBeNull();
  });

  it('returns null for a stored value the schema rejects', () => {
    window.sessionStorage.setItem(confirmDraftKey('club-norte'), '{"not":"a draft"}');

    expect(readConfirmDraft('club-norte')).toBeNull();
  });

  it('is removed by clearConfirmDraft', () => {
    saveConfirmDraft('club-norte', DRAFT);

    clearConfirmDraft('club-norte');

    expect(readConfirmDraft('club-norte')).toBeNull();
  });
});

describe('booking result', () => {
  it('returns the result that was saved for the same slug', () => {
    saveBookingResult('club-norte', { token: 'tok-1' });

    expect(readBookingResult('club-norte')).toEqual({ token: 'tok-1' });
  });

  it('returns null when the stored result has no token', () => {
    window.sessionStorage.setItem(bookingResultKey('club-norte'), '{"bookingInfo":{}}');

    expect(readBookingResult('club-norte')).toBeNull();
  });

  it('is removed by clearBookingResult, so a later return cannot pick it up', () => {
    saveBookingResult('club-norte', { token: 'tok-1' });

    clearBookingResult('club-norte');

    expect(readBookingResult('club-norte')).toBeNull();
  });

  it('is not the same entry as the confirm draft of the same slug', () => {
    saveConfirmDraft('club-norte', DRAFT);

    expect(readBookingResult('club-norte')).toBeNull();
    expect(safeSessionStorage.get(confirmDraftKey('club-norte'))).not.toBeNull();
  });
});

describe('hold-expired notice', () => {
  it('waits until cleared', () => {
    expect(hasHoldExpiredNotice('club-norte')).toBe(false);
    saveHoldExpiredNotice('club-norte');
    expect(hasHoldExpiredNotice('club-norte')).toBe(true);
    clearHoldExpiredNotice('club-norte');
    expect(hasHoldExpiredNotice('club-norte')).toBe(false);
  });

  it('is kept per slug', () => {
    saveHoldExpiredNotice('club-norte');
    expect(hasHoldExpiredNotice('club-sur')).toBe(false);
  });
});

describe('confirm draft hold', () => {
  it('rejects a draft without a hold token', () => {
    const { holdToken: _holdToken, ...withoutHold } = DRAFT;
    safeSessionStorage.set(confirmDraftKey('club-norte'), JSON.stringify(withoutHold));
    expect(readConfirmDraft('club-norte')).toBeNull();
  });
});
