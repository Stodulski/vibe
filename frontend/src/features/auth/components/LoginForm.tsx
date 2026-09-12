import { useRef, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { loginSchema, type LoginDto } from '../schemas/auth.schemas';
import { useLogin } from '../hooks/useLogin';
import { ES_AR } from '@/shared/i18n/es_AR';
import { submitHandler } from '@/shared/lib/form';
import { useTurnstileChallenge } from '@/shared/hooks/useTurnstileChallenge';
import { TurnstileField, type TurnstileFieldHandle } from '@/shared/components/common/TurnstileField';
import { LoginEmailField } from './login-form/LoginEmailField';
import { LoginPasswordField } from './login-form/LoginPasswordField';
import { LoginFormFooter } from './login-form/LoginFormFooter';

const t = ES_AR;

export function LoginForm() {
  const turnstileRef = useRef<TurnstileFieldHandle>(null);
  const turnstile = useTurnstileChallenge();
  const login = useLogin({ resetTurnstile: () => turnstileRef.current?.reset() });
  const [showPassword, setShowPassword] = useState(false);

  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useForm<LoginDto>({
    resolver: zodResolver(loginSchema),
  });

  const onSubmit = (data: LoginDto) => {
    login.mutate({ ...data, ...turnstile.payload() });
  };

  return (
    <div className="auth-card w-full">
      {/* Header */}
      <div className="flex flex-col items-center px-0 pt-2 pb-1 md:pt-7">
        <h1 className="font-display text-text-primary text-xl font-bold tracking-tight sm:text-2xl">{t.auth.login}</h1>
      </div>

      {/* Form */}
      <div className="px-0 pt-5 pb-7">
        <form onSubmit={submitHandler(handleSubmit, onSubmit)} className="space-y-4" noValidate>
          <LoginEmailField register={register} errors={errors} />
          <LoginPasswordField
            register={register}
            errors={errors}
            showPassword={showPassword}
            onToggleShowPassword={() => {
              setShowPassword(!showPassword);
            }}
          />

          <TurnstileField ref={turnstileRef} onTokenChange={turnstile.onTokenChange} />

          <LoginFormFooter isPending={login.isPending} submitDisabled={turnstile.isBlocked(login.isPending)} />
        </form>
      </div>
    </div>
  );
}
