import { createRegistrationTokenStorage } from './registrationTokenStorage';

const storage = createRegistrationTokenStorage('vibe_event_registration');

export const storeEventRegistrationToken = storage.store;
export const readEventRegistrationToken = storage.read;
export const clearEventRegistrationToken = storage.clear;
