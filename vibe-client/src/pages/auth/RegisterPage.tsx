import { RegisterForm } from '@/features/auth';
import { AuthSplitLayout } from '@/shared/components/layout/AuthSplitLayout';
import { usePageTitle, useOGTags } from '@/shared/hooks/usePageTitle';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export default function RegisterPage() {
  usePageTitle(t.auth.register);
  useOGTags({
    title: t.auth.registerMetaTitle,
    description: t.auth.registerMetaDescription,
  });

  return (
    <AuthSplitLayout>
      <RegisterForm />
    </AuthSplitLayout>
  );
}
