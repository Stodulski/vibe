import type { Body, Ok, Spec } from './spec';

// ─── Open-play events (canchas libres) ───

/** An event that holds its courts for a window while people sign up one by one. `end_time` is earlier than `start_time` when it runs past midnight. */
export type OpenPlayEvent = Spec<'OpenPlayEvent'>;

/** Someone who held a spot, with the payment carrying their money. */
export type OpenPlayParticipant = Spec<'OpenPlayParticipant'>;

/** The hours an event holds on one court, as the owner's grid draws them. */
export type EventBlock = Spec<'EventBlock'>;

export type CreateOpenPlayEventRequest = Body<'eventsCreate'>;

export type OpenPlayEventDetailResponse = Ok<'eventsGet'>;

export type CancelOpenPlayEventResponse = Ok<'eventsCancel'>;
