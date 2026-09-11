import { useEffect, useRef } from 'react';
import { useMPConnect } from '@/features/complex';
import { MPConnectStep } from './MPConnectStep';

interface MPConnectStepContainerProps {
  complexId: string;
  completeOnboarding: (id: string) => void;
  onBack: () => void;
}

/**
 * Wires the presentational `MPConnectStep` to its data — split out so
 * `MPConnectStep` itself stays a plain, easily-tested component. Only ever
 * mounted for step 3 with a real `complexId` (see `OnboardingStepContent`),
 * so the status query stays enabled and polls every 3s while the owner is
 * off completing the MercadoPago handshake in another tab.
 */
export function MPConnectStepContainer({ complexId, completeOnboarding, onBack }: MPConnectStepContainerProps) {
  const { connected, authUrl, handleConnectClick } = useMPConnect(complexId, {
    refetchInterval: 3000,
  });
  const wasConnected = useRef(false);

  // If MP transitions from disconnected -> connected, complete onboarding.
  useEffect(() => {
    if (connected && !wasConnected.current) {
      completeOnboarding(complexId);
    }
    wasConnected.current = connected;
  }, [complexId, connected, completeOnboarding]);

  return (
    <MPConnectStep
      complexId={complexId}
      mpConnected={connected}
      mpAuthUrl={authUrl}
      onConnectClick={handleConnectClick}
      onBack={onBack}
      onFinish={() => {
        completeOnboarding(complexId);
      }}
    />
  );
}
