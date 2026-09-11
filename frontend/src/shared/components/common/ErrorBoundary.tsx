import { Component, type ErrorInfo, type ReactNode } from 'react';
import * as Sentry from '@sentry/react';
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

interface Props {
  children: ReactNode;
  fallback?: ReactNode;
}

interface State {
  hasError: boolean;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  override componentDidCatch(error: Error, info: ErrorInfo) {
    Sentry.captureException(error, { extra: { componentStack: info.componentStack } });
  }

  handleRetry = () => {
    this.setState({ hasError: false });
  };

  override render() {
    if (this.state.hasError) {
      if (this.props.fallback) return this.props.fallback;

      return (
        <div
          className="flex min-h-[400px] items-center justify-center p-4 animate-fade-in sm:p-8"
          role="alert"
          aria-live="assertive"
        >
          <div className="w-full max-w-sm rounded-2xl border border-error-border/30 bg-bg-subtle p-6 text-center shadow-lg shadow-black/10 sm:p-10">
            <div className="mx-auto mb-6 flex size-14 items-center justify-center rounded-2xl bg-error-bg ring-4 ring-error-border/10">
              <AlertTriangle className="size-7 text-error-icon" aria-hidden="true" />
            </div>
            <h2 className="text-lg font-bold text-text-primary">{t.layout.errorTitle}</h2>
            <p className="mt-2 text-sm leading-relaxed text-text-secondary">{t.layout.errorDescription}</p>
            <div className="mt-8 flex flex-col-reverse items-center justify-center gap-3 sm:flex-row sm:gap-4">
              <Button variant="outline" onClick={this.handleRetry} className="w-full rounded-xl sm:w-auto">
                {t.layout.retry}
              </Button>
              <Button onClick={() => (window.location.href = '/')} className="w-full rounded-xl sm:w-auto">
                {t.layout.backHome}
              </Button>
            </div>
          </div>
        </div>
      );
    }

    return this.props.children;
  }
}
