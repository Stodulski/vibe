// The events feature's public API for the app layer (the events page composes it
// with the tournaments feature). Route skeletons import their pieces directly, so
// that the main bundle never pulls in these components.
export { useEvents } from './hooks/useEvents';
export { upcomingRange as upcomingEventsRange, pastRange as pastEventsRange } from './lib/eventRanges';
export { EVENT_LIST_CLASS, EVENTS_PAGE_CLASS } from './lib/eventLayout';
export { CreateEventModal } from './components/CreateEventModal';
export { EventCard } from './components/EventCard';
export { EventDetail } from './components/EventDetail';
export { EventListSkeleton } from './components/EventListSkeleton';
export { EventTypePicker } from './components/EventTypePicker';
export type { CreateEventKind } from './components/EventTypePicker';
