import type { Body, Ok, Spec } from './spec';
import type { Booking } from './booking';

// ─── Client ───

export type Client = Spec<'Client'>;

export type ClientsListResponse = Ok<'clientsList'>;

export type ClientDetailResponse = Omit<Ok<'clientsGet'>, 'recent_bookings'> & { recent_bookings: Booking[] };

export type UpdateClientRequest = Body<'clientsUpdate'>;
