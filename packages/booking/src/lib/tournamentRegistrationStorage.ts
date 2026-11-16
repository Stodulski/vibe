import { createRegistrationTokenStorage } from './registrationTokenStorage';

const storage = createRegistrationTokenStorage('vibe_tournament_registration');

export const storeTournamentRegistrationToken = storage.store;
export const readTournamentRegistrationToken = storage.read;
export const clearTournamentRegistrationToken = storage.clear;
