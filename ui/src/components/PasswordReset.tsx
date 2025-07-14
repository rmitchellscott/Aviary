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

  const requestReset = async () => {
    if (!email) {
      setMessage({ type: 'error', text: 'Please enter your email address' });
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
          text: 'If the email exists, a password reset link has been sent' 
        });
      } else {
        setMessage({ type: 'error', text: data.error || 'Failed to send reset email' });
      }
    } catch (error) {
      setMessage({ type: 'error', text: 'Network error. Please try again.' });
    } finally {
      setLoading(false);
    }
  };

  const confirmReset = async () => {
    if (!token || !newPassword || !confirmPassword) {
      setMessage({ type: 'error', text: 'Please fill in all fields' });
      return;
    }

    if (newPassword !== confirmPassword) {
      setMessage({ type: 'error', text: 'Passwords do not match' });
      return;
    }

    if (newPassword.length < 8) {
      setMessage({ type: 'error', text: 'Password must be at least 8 characters long' });
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
          text: 'Password has been reset successfully. You can now log in with your new password.' 
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
        setMessage({ type: 'error', text: data.error || 'Failed to reset password' });
      }
    } catch (error) {
      setMessage({ type: 'error', text: 'Network error. Please try again.' });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-background flex items-center justify-center p-4">
      <Card className="w-full max-w-md">
        <CardHeader className="text-center">
          <CardTitle className="flex items-center justify-center gap-2">
            <Lock className="h-5 w-5" />
            {step === 'request' ? 'Reset Password' : 'Set New Password'}
          </CardTitle>
        </CardHeader>
        
        <CardContent className="space-y-4">
          {message && (
            <Alert className={message.type === 'error' ? 'border-destructive' : 'border-green-500'}>
              <div className="flex items-center gap-2">
                {message.type === 'success' ? (
                  <CheckCircle className="h-4 w-4 text-green-600" />
                ) : (
                  <XCircle className="h-4 w-4 text-destructive" />
                )}
                <AlertDescription className={message.type === 'error' ? 'text-destructive' : 'text-green-700'}>
                  {message.text}
                </AlertDescription>
              </div>
            </Alert>
          )}

          {step === 'request' ? (
            <>
              <div className="space-y-2">
                <Label htmlFor="email">Email Address</Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="Enter your email address"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  disabled={loading}
                />
              </div>

              <div className="space-y-2">
                <Button 
                  onClick={requestReset} 
                  disabled={loading || !email}
                  className="w-full"
                >
                  <Mail className="h-4 w-4 mr-2" />
                  {loading ? 'Sending...' : 'Send Reset Email'}
                </Button>
                
                <Button 
                  variant="outline" 
                  onClick={onBack}
                  className="w-full"
                  disabled={loading}
                >
                  Back to Login
                </Button>
              </div>

              <div className="text-center">
                <p className="text-sm text-muted-foreground">
                  Enter your email address and we'll send you a link to reset your password.
                </p>
              </div>
            </>
          ) : (
            <>
              <div className="space-y-2">
                <Label htmlFor="token">Reset Token</Label>
                <Input
                  id="token"
                  type="text"
                  placeholder="Enter reset token from email"
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                  disabled={loading}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="new-password">New Password</Label>
                <Input
                  id="new-password"
                  type="password"
                  placeholder="Enter new password"
                  value={newPassword}
                  onChange={(e) => setNewPassword(e.target.value)}
                  disabled={loading}
                />
              </div>

              <div className="space-y-2">
                <Label htmlFor="confirm-password">Confirm New Password</Label>
                <Input
                  id="confirm-password"
                  type="password"
                  placeholder="Confirm new password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  disabled={loading}
                />
              </div>

              <div className="space-y-2">
                <Button 
                  onClick={confirmReset} 
                  disabled={loading || !token || !newPassword || !confirmPassword}
                  className="w-full"
                >
                  <Lock className="h-4 w-4 mr-2" />
                  {loading ? 'Resetting...' : 'Reset Password'}
                </Button>
                
                <Button 
                  variant="outline" 
                  onClick={() => setStep('request')}
                  className="w-full"
                  disabled={loading}
                >
                  Back to Email Entry
                </Button>
              </div>

              <div className="text-center">
                <p className="text-sm text-muted-foreground">
                  Password must be at least 8 characters long.
                </p>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}