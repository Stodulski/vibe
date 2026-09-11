import { useEffect } from 'react';

/** Adds a `<meta name="robots" content="noindex, nofollow">` tag for the lifetime of the component. */
export function useNoIndex() {
  useEffect(() => {
    const meta = document.createElement('meta');
    meta.name = 'robots';
    meta.content = 'noindex, nofollow';
    document.head.appendChild(meta);
    return () => {
      document.head.removeChild(meta);
    };
  }, []);
}
