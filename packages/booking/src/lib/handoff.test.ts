import { describe, it, expect, beforeEach } from 'vitest';
import { safeSessionStorage } from '@vibe/ui';
import type { BookingSlotInfo } from '../components/booking-form/types';
import {
  bookingResultKey,
  clearBookingResult,
  clearConfirmDraft,
  confirmDraftKey,
  readBookingResult,
  readConfirmDraft,
  saveBookingResult,
  saveConfirmDraft,
} from './handoff';

const DRAFT: BookingSlotInfo = {
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
