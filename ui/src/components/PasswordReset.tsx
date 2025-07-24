import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Mail, Lock, CheckCircle, XCircle } from 'lucide-react';

interface PasswordResetProps {
  onBack: () => void;
}

export function PasswordReset({ onBack }: PasswordResetProps) {
  const { t } = useTranslation();
  const [step, setStep] = useState<'request' | 'confirm'>('request');
  const [email, setEmail] = useState('');
  const [token, setToken] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null);

  // Get token from URL if present
  React.useEffect(() => {
    const urlParams = new URLSearchParams(window.location.search);
    const tokenParam = urlParams.get('token');
    if (tokenParam) {
      setToken(tokenParam);
      setStep('confirm');
    }
  }, []);

  const requestReset = async (e?: React.FormEvent) => {
    if (e) {
      e.preventDefault();
    }
    
    if (!email) {
      setMessage({ type: 'error', text: t('register.missing_fields') });
      return;
    }

    setLoading(true);
    setMessage(null);

    try {
      const response = await fetch('/api/auth/password-reset', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ email }),
      });

      const data = await response.json();

      if (response.ok) {
        setMessage({ 
          type: 'success', 
          text: t('password_reset.email_sent', 'If the email exists, a password reset link has been sent') 
        });
      } else {
        setMessage({ type: 'error', text: data.error || t('password_reset.send_email_error', 'Failed to send reset email') });
      }
    } catch (error) {
      setMessage({ type: 'error', text: t('register.network_error') });
    } finally {
      setLoading(false);
    }
  };

  const confirmReset = async (e?: React.FormEvent) => {
    if (e) {
      e.preventDefault();
    }
    
    if (!token || !newPassword || !confirmPassword) {
      setMessage({ type: 'error', text: t('register.missing_fields') });
      return;
    }

    if (newPassword !== confirmPassword) {
      setMessage({ type: 'error', text: t('register.password_mismatch') });
      return;
    }

    if (newPassword.length < 8) {
      setMessage({ type: 'error', text: t('register.password_too_short') });
      return;
    }

    setLoading(true);
    setMessage(null);

    try {
      const response = await fetch('/api/auth/password-reset/confirm', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          token,
          password: newPassword,
        }),
      });

      const data = await response.json();

      if (response.ok) {
        setMessage({ 
          type: 'success', 
          text: t('password_reset.success', 'Password has been reset successfully. You can now log in with your new password.') 
        });
        // Clear the form
        setToken('');
        setNewPassword('');
        setConfirmPassword('');
        
        // Redirect to login after a delay
        setTimeout(() => {
          window.location.href = '/';
        }, 3000);
      } else {
        setMessage({ type: 'error', text: data.error || t('password_reset.reset_error', 'Failed to reset password') });
      }
    } catch (error) {
      setMessage({ type: 'error', text: t('register.network_error') });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-background pt-0 pb-8 px-8">
      <Card className="max-w-md mx-auto bg-card">
        <CardHeader>
          <CardTitle className="text-xl flex items-center gap-2">
            <Lock className="h-5 w-5" />
            {step === 'request' ? t('password_reset.title') : t('password_reset.new_password_title')}
          </CardTitle>
        </CardHeader>
        
        <CardContent className="space-y-4">
          {message && (
            <Alert variant={message.type === 'error' ? 'destructive' : 'default'}>
              <div className="flex items-center gap-3">
                {message.type === 'success' ? (
                  <CheckCircle className="h-4 w-4 flex-shrink-0" />
                ) : (
                  <XCircle className="h-4 w-4 flex-shrink-0" />
                )}
                <AlertDescription className="flex-1">
                  {message.text}
                </AlertDescription>
              </div>
            </Alert>
          )}

          {step === 'request' ? (
            <form onSubmit={requestReset}>
              <div className="space-y-2">
                <Label htmlFor="email">{t('register.email')}</Label>
                <Input
                  id="email"
                  type="email"
                  placeholder={t('password_reset.email_placeholder')}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  disabled={loading}
                  autoFocus
                />
              </div>

              <div className="flex flex-col space-y-4 mt-4">
                <Button 
                  type="submit"
                  disabled={loading || !email}
                  className="w-full"
                >
                  <Mail className="h-4 w-4 mr-2" />
                  {loading ? t('password_reset.sending') : t('password_reset.send_email')}
                </Button>
                
                <div className="text-center">
                  <Button 
                    type="button"
                    variant="ghost" 
                    size="sm"
                    onClick={onBack}
                    className="text-sm text-muted-foreground hover:text-foreground hover:bg-muted"
                    disabled={loading}
                  >
                    {t('register.back_to_login')}
                  </Button>
                </div>
              </div>

              <div className="text-center mt-4">
                <p className="text-sm text-muted-foreground">
                  {t('password_reset.email_instruction', 'Enter your email address and we\'ll send you a link to reset your password.')}
                </p>
              </div>
            </form>
          ) : (
            <form onSubmit={confirmReset} className="space-y-4">
              {/* Hidden token field - token is automatically populated from URL */}
              <input type="hidden" value={token} />

              <div className="space-y-2">
                <Label htmlFor="new-password">{t('admin.labels.new_password')}</Label>
                <Input
                  id="new-password"
                  type="password"
                  placeholder={t('password_reset.password_placeholder')}
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  disabled={loading}
                  autoFocus
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="confirm-password">{t('register.confirm_password')}</Label>
                <Input
                  id="confirm-password"
                  type="password"
                  placeholder={t('password_reset.confirm_placeholder')}
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  disabled={loading}
                />
              </div>

              <div className="space-y-2">
                <Button 
                  type="submit"
                  disabled={loading || !token || !newPassword || !confirmPassword}
                  className="w-full"
                >
                  <Lock className="h-4 w-4 mr-2" />
                  {loading ? t('password_reset.resetting') : t('password_reset.reset_button')}
                </Button>
                
                <Button 
                  type="button"
                  variant="outline" 
                  onClick={onBack}
                  className="w-full"
                  disabled={loading}
                >
                  {t('register.back_to_login')}
                </Button>
              </div>

              <div className="text-center">
                <p className="text-sm text-muted-foreground">
                  {t('register.password_help')}
                </p>
              </div>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  );
}