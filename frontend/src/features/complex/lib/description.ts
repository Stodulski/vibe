/**
 * The longest description the API stores, in characters (`descriptionMaxLen`
 * in backend/internal/complexes/handlers.go).
 */
export const DESCRIPTION_MAX_LENGTH = 600;

/**
 * Shortest description that counts as written in the profile checklist. A
 * line or two gives search engines too little text to say what the venue is,
 * so anything under one full sentence is still "to do".
 */
export const MIN_DESCRIPTION_LENGTH = 80;
