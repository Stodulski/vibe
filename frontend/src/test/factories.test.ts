import { describe, it, expect } from 'vitest';
import { makeUser, makeCourt, makeBooking, makeComplex, makeClient } from './factories';

describe('makeUser', () => {
  it('returns a User with sensible defaults', () => {
    const user = makeUser();
    expect(user.role).toBe('owner');
    expect(user.email).toBe('user@test.com');
  });

  it('applies overrides on top of the defaults', () => {
    const user = makeUser({ role: 'superadmin', id: 'admin-1' });
    expect(user.role).toBe('superadmin');
    expect(user.id).toBe('admin-1');
    expect(user.email).toBe('user@test.com');
  });
});

describe('makeCourt', () => {
  it('returns a Court with sensible defaults', () => {
    const court = makeCourt();
    expect(court.sport).toBe('padel');
    expect(court.is_active).toBe(true);
  });

  it('applies overrides on top of the defaults', () => {
    const court = makeCourt({ id: '1', name: 'Court 1' });
    expect(court.id).toBe('1');
    expect(court.name).toBe('Court 1');
    expect(court.sport).toBe('padel');
  });
});

describe('makeBooking', () => {
  it('returns a Booking with sensible defaults', () => {
    const booking = makeBooking();
    expect(booking.status).toBe('confirmed');
    expect(booking.collection_status).toBe('unpaid');
    expect(booking.refund_status).toBe('none');
  });

  it('applies overrides on top of the defaults', () => {
    const booking = makeBooking({ id: '1' });
    expect(booking.id).toBe('1');
    expect(booking.status).toBe('confirmed');
  });
});

describe('makeComplex', () => {
  it('returns a Complex with sensible defaults', () => {
    const complex = makeComplex();
    expect(complex.currency).toBe('ARS');
    expect(complex.is_active).toBe(true);
  });

  it('applies overrides on top of the defaults', () => {
    const complex = makeComplex({ id: 'c1', name: 'Test' });
    expect(complex.id).toBe('c1');
    expect(complex.name).toBe('Test');
    expect(complex.currency).toBe('ARS');
  });
});

describe('makeClient', () => {
  it('returns a Client with sensible defaults', () => {
    const client = makeClient();
    expect(client.is_blocked).toBe(false);
    expect(client.total_bookings).toBe(0);
  });

  it('applies overrides on top of the defaults', () => {
    const client = makeClient({ id: 'cl2', first_name: 'Ana' });
    expect(client.id).toBe('cl2');
    expect(client.first_name).toBe('Ana');
    expect(client.is_blocked).toBe(false);
  });
});
