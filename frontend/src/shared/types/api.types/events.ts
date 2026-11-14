import type { Body, Ok, Open, Spec } from './spec';

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

// ─── Public side of the events, as the booking package reads them ───

/**
 * An event as a visitor sees it. `sport` and `status` are read through
 * {@link Open}: an event in a sport or state this build predates must cost
 * that one card its label, not the whole list.
 */
export type PublicOpenPlayEvent = Omit<Spec<'PublicOpenPlayEvent'>, 'sport' | 'status'> & {
  sport: Open<Spec<'PublicOpenPlayEvent'>['sport']>;
  status: Open<Spec<'PublicOpenPlayEvent'>['status']>;
};

/** `POST .../registrations`: the held spot and the checkout that pays for it. */
export type PublicEventSignUp = Omit<Spec<'PublicEventSignUp'>, 'event'> & { event: PublicOpenPlayEvent };

/** Whether and how a person can still cancel their own spot. */
export type PublicEventCancellation = Spec<'PublicEventRegistration'>['cancellation'];

/** A person's own spot, read through the token the checkout returns them with. */
export type PublicEventRegistration = Omit<Spec<'PublicEventRegistration'>, 'event' | 'status'> & {
  event: PublicOpenPlayEvent;
  status: Open<Spec<'PublicEventRegistration'>['status']>;
};
