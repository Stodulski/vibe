import { useParams } from 'react-router-dom';
import { HTTPError } from 'ky';
import { useComplexBySlug, useAvailability } from '@/features/public-booking';
import { useComplexPageState } from './complex-page/useComplexPageState';
import { useComplexPageMeta } from './complex-page/useComplexPageMeta';
import { useSlotInvalidation, isStartTimeStillOffered } from './complex-page/useSlotInvalidation';
import { useAvailableSports } from './complex-page/useAvailableSports';
import { useFilteredCourts } from './complex-page/useFilteredCourts';
import { useHandleContinue } from './complex-page/useHandleContinue';
import { NotFoundState } from './complex-page/NotFoundState';
import { ComplexErrorState } from './complex-page/ComplexErrorState';
import { ComplexPageLoadingSkeleton } from './complex-page/ComplexPageLoadingSkeleton';
import { ComplexPageContent } from './complex-page/ComplexPageContent';

// 404 means the slug never resolved to a complex; anything else (500, a
// dropped connection) is recoverable, so it gets a retry instead of the same
// dead-end "not found" message.
function ComplexLoadError({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const notFound = error instanceof HTTPError && error.response.status === 404;
  if (notFound) {
    return <NotFoundState />;
  }
  return <ComplexErrorState onRetry={onRetry} />;
}

/** Availability, the slots derived from it, and the "which court?" question it answers away when its hour stops being offered. */
function useComplexAvailability(slug: string | undefined, state: ReturnType<typeof useComplexPageState>) {
  const {
    data: availability,
    isLoading: availLoading,
    isPlaceholderData: availStale,
    isError: availError,
    refetch: refetchAvailability,
  } = useAvailability(slug, state.debouncedDateStr, state.duration);

  const activeSlot = useSlotInvalidation(state.selectedSlot, availability);
  const filteredCourts = useFilteredCourts(availability, state.sportFilter);
  // The open "which court?" question answers itself away when its hour is no
  // longer on offer — derived here (not written back up from `CourtSelector`,
  // which owns none of this state).
  const activePendingStartTime = isStartTimeStillOffered(state.pendingStartTime, filteredCourts)
    ? state.pendingStartTime
    : null;

  return {
    availability,
    availLoading,
    availStale,
    availError,
    refetchAvailability,
    activeSlot,
    filteredCourts,
    activePendingStartTime,
  };
}

function ComplexPageLoadGuard({
  isLoading,
  error,
  complexData,
  onRetryComplex,
}: {
  isLoading: boolean;
  error: unknown;
  complexData: unknown;
  onRetryComplex: () => void;
}) {
  if (isLoading) {
    return <ComplexPageLoadingSkeleton />;
  }
  if (error) {
    return <ComplexLoadError error={error} onRetry={onRetryComplex} />;
  }
  if (!complexData) {
    return <NotFoundState />;
  }
  return null;
}

function useComplexContinue(
  slug: string | undefined,
  complexData: ReturnType<typeof useComplexBySlug>['data'],
  activeSlot: ReturnType<typeof useSlotInvalidation>,
  dateStr: string,
) {
  const mpConnected = !!complexData?.complex.payments_enabled;
  const handleContinue = useHandleContinue(slug, complexData?.complex, mpConnected, activeSlot, dateStr);
  return { mpConnected, handleContinue };
}

/**
 * Just the content JSX, pulled out of `ComplexPage` to keep that component
 * under the file's line-count lint budget — every hook stays called
 * unconditionally in `ComplexPage`, before the loading guard, so this split
 * changes nothing about when availability starts fetching.
 */
function ComplexPageBody({
  complexData,
  state,
  availableSports,
  availabilityBag,
  mpConnected,
  handleContinue,
}: {
  complexData: NonNullable<ReturnType<typeof useComplexBySlug>['data']>;
  state: ReturnType<typeof useComplexPageState>;
  availableSports: ReturnType<typeof useAvailableSports>;
  availabilityBag: ReturnType<typeof useComplexAvailability>;
  mpConnected: boolean;
  handleContinue: ReturnType<typeof useComplexContinue>['handleContinue'];
}) {
  const { complex, schedules } = complexData;
  const {
    availability,
    availLoading,
    availStale,
    availError,
    refetchAvailability,
    activeSlot,
    filteredCourts,
    activePendingStartTime,
  } = availabilityBag;

  return (
    <ComplexPageContent
      complex={complex}
      schedules={schedules}
      mpConnected={mpConnected}
      selectedDate={state.selectedDate}
      onDateSelect={state.handleDateSelect}
      availableSports={availableSports}
      sportFilter={state.sportFilter}
      onSportFilterChange={state.handleSportFilter}
      availLoading={availLoading}
      availError={availError}
      onAvailRetry={() => {
        void refetchAvailability();
      }}
      availStale={availStale}
      availability={availability}
      dateStr={state.dateStr}
      filteredCourts={filteredCourts}
      selectedSlot={activeSlot}
      onSelectSlot={state.setSelectedSlot}
      onContinue={handleContinue}
      duration={state.duration}
      selectedDuration={state.selectedDuration}
      onDurationChange={state.handleDurationChange}
      pendingStartTime={activePendingStartTime}
      onPendingStartTimeChange={state.setPendingStartTime}
      answeredFromUrl={state.answeredFromUrl}
    />
  );
}

export default function ComplexPage() {
  const { slug } = useParams<{ slug: string }>();
  const state = useComplexPageState();

  const { data: complexData, isLoading, error, refetch: refetchComplex } = useComplexBySlug(slug);
  useComplexPageMeta(slug, complexData);
  const availableSports = useAvailableSports(complexData);

  const availabilityBag = useComplexAvailability(slug, state);
  const { mpConnected, handleContinue } = useComplexContinue(
    slug,
    complexData,
    availabilityBag.activeSlot,
    state.dateStr,
  );

  if (isLoading || error || !complexData) {
    return (
      <ComplexPageLoadGuard
        isLoading={isLoading}
        error={error}
        complexData={complexData}
        onRetryComplex={() => {
          void refetchComplex();
        }}
      />
    );
  }

  return (
    <ComplexPageBody
      complexData={complexData}
      state={state}
      availableSports={availableSports}
      availabilityBag={availabilityBag}
      mpConnected={mpConnected}
      handleContinue={handleContinue}
    />
  );
}
