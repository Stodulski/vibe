import { Copy, Check } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import { PUBLIC_LINK_BAR_CLASS } from '../../components/dashboardLayout';

const t = ES_AR;

interface PublicLinkBarProps {
  publicUrl: string;
  copied: boolean;
  onCopy: () => void;
}

export function PublicLinkBar({ publicUrl, copied, onCopy }: PublicLinkBarProps) {
  // The protocol is chrome, not information — dropping it from what's shown
  // (the copy button still puts the real, complete URL on the clipboard)
  // frees up room for the part that's actually distinguishing: the slug.
  const displayUrl = publicUrl.replace(/^https?:\/\//, '');

  return (
    <div className={PUBLIC_LINK_BAR_CLASS}>
      <span className="text-primary-400 min-w-0 truncate text-sm" title={publicUrl}>
        {displayUrl}
      </span>
      {/* Sized to the line of text it sits next to; touch devices still get
          the 44px target from the global pointer:coarse rule. */}
      <button
        onClick={onCopy}
        className="border-border-subtle bg-bg-base text-primary-400 hover:bg-bg-overlay inline-flex h-8 shrink-0 items-center gap-1.5 rounded-full border px-2.5 text-sm font-medium transition-colors"
      >
        {copied ? <Check className="text-success-text size-3.5" /> : <Copy className="text-primary-400 size-3.5" />}
        <span className="hidden sm:inline">{t.dashboard.copyLink}</span>
        <span className="sr-only sm:hidden">{t.dashboard.copyLink}</span>
      </button>
    </div>
  );
}
