"use client";

import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { ArrowLeft, UserPlus } from "lucide-react";

export function RegisterForm() {
  const { t } = useTranslation();
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState(false);
  const [registrationEnabled, setRegistrationEnabled] = useState(false);

  useEffect(() => {
    const checkRegistrationStatus = async () => {
      try {
        const response = await fetch("/api/auth/registration-status", {
          credentials: "include",
        });
        if (response.ok) {
          const data = await response.json();
          setRegistrationEnabled(data.enabled || false);
        }
      } catch (error) {
        console.error("Failed to check registration status:", error);
        // Default to disabled if we can't check
        setRegistrationEnabled(false);
      }
    };

    checkRegistrationStatus();
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError("");
    setSuccess(false);

    // Client-side validation
    if (!username || !email || !password || !confirmPassword) {
      setError(t("register.missing_fields"));
      setLoading(false);
      return;
    }

    if (password !== confirmPassword) {
      setError(t("register.password_mismatch"));
      setLoading(false);
      return;
    }

    if (password.length < 8) {
      setError(t("register.password_too_short"));
      setLoading(false);
      return;
    }

    try {
      const response = await fetch("/api/auth/register/public", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ username, email, password }),
        credentials: "include",
      });

      if (response.ok) {
        setSuccess(true);
        setUsername("");
        setEmail("");
        setPassword("");
        setConfirmPassword("");
      } else {
        const data = await response.json();
        if (data.error) {
          // Handle specific error messages
          if (data.error.includes("already exists")) {
            setError(t("register.user_exists"));
          } else if (data.error.includes("disabled")) {
            setError(t("register.disabled"));
          } else {
            setError(t(data.error) || data.error);
          }
        } else {
          setError(t("register.fail"));
        }
      }
    } catch {
      setError(t("register.network_error"));
    } finally {
      setLoading(false);
    }
  };

  if (!registrationEnabled) {
    return (
      <div className="bg-background pt-0 pb-8 px-8">
        <Card className="max-w-md mx-auto bg-card">
          <CardHeader>
            <CardTitle className="text-xl">{t("register.title")}</CardTitle>
          </CardHeader>
          <CardContent>
            <Alert>
              <AlertDescription>
                {t("register.disabled_message")}
              </AlertDescription>
            </Alert>
            <div className="mt-4">
              <Button 
                variant="outline" 
                className="w-full"
                onClick={() => window.location.href = '/login'}
              >
                <ArrowLeft className="mr-2 h-4 w-4" />
                {t("register.back_to_login")}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (success) {
    return (
      <div className="bg-background pt-0 pb-8 px-8">
        <Card className="max-w-md mx-auto bg-card">
          <CardHeader>
            <CardTitle className="text-xl">{t("register.success_title")}</CardTitle>
          </CardHeader>
          <CardContent>
            <Alert>
              <AlertDescription>
                {t("register.success_message")}
              </AlertDescription>
            </Alert>
            <div className="mt-4">
              <Button 
                className="w-full"
                onClick={() => window.location.href = '/login'}
              >
                {t("register.login_now")}
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="bg-background pt-0 pb-8 px-8">
      <Card className="max-w-md mx-auto bg-card">
        <CardHeader>
          <CardTitle className="text-xl flex items-center">
            <UserPlus className="mr-2 h-5 w-5" />
            {t("register.title")}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <Label htmlFor="username" className="mb-2 block">
                {t("register.username")}
              </Label>
              <Input
                id="username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                disabled={loading}
                minLength={3}
                maxLength={50}
                pattern="[a-zA-Z0-9_-]+"
                title={t("register.username_help")}
              />
            </div>
            <div>
              <Label htmlFor="email" className="mb-2 block">
                {t("register.email")}
              </Label>
              <Input
                id="email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                disabled={loading}
              />
            </div>
            <div>
              <Label htmlFor="password" className="mb-2 block">
                {t("register.password")}
              </Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                disabled={loading}
                minLength={8}
              />
              <p className="text-sm text-muted-foreground mt-1">
                {t("register.password_help")}
              </p>
            </div>
            <div>
              <Label htmlFor="confirmPassword" className="mb-2 block">
                {t("register.confirm_password")}
              </Label>
              <Input
                id="confirmPassword"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
                disabled={loading}
                minLength={8}
              />
            </div>
            {error && (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            <div className="flex flex-col space-y-4">
              <Button type="submit" disabled={loading} className="w-full">
                {loading ? t("register.creating") : t("register.button")}
              </Button>
              <Button 
                variant="outline" 
                onClick={() => window.location.href = '/login'}
                className="w-full"
              >
                {t("register.already_have_account")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
