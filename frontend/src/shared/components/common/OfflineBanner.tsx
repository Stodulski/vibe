import { useState, useEffect } from 'react';
import { WifiOff } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function OfflineBanner() {
  const [isOffline, setIsOffline] = useState(!navigator.onLine);

  useEffect(() => {
    const goOffline = () => {
      setIsOffline(true);
    };
    const goOnline = () => {
      setIsOffline(false);
    };
    window.addEventListener('offline', goOffline);
    window.addEventListener('online', goOnline);
    return () => {
      window.removeEventListener('offline', goOffline);
      window.removeEventListener('online', goOnline);
    };
  }, []);

  if (!isOffline) return null;

  return (
    <div className="fixed bottom-4 left-1/2 z-50 -translate-x-1/2 animate-fade-in" role="alert">
      <div className="flex items-center gap-2 rounded-full border border-warning-border bg-warning-bg px-4 py-2 text-sm font-medium text-warning-text shadow-lg">
        <WifiOff className="size-4 shrink-0" />
        {t.layout.offline}
      </div>
    </div>
  );
}
