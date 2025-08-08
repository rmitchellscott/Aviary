"use client";

import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useAuth } from "@/components/AuthProvider";
import { useConfig } from "@/components/ConfigProvider";

interface LoginFormProps {
  onLogin: () => void;
}

export function LoginForm({ onLogin }: LoginFormProps) {
  const { t } = useTranslation();
  const { multiUserMode } = useAuth();
  const { config } = useConfig();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [registrationEnabled, setRegistrationEnabled] = useState(false);
  
  const smtpConfigured = config?.smtpConfigured || false;
  const oidcEnabled = config?.oidcEnabled || false;
  const oidcSsoOnly = config?.oidcSsoOnly || false;
  const oidcButtonText = config?.oidcButtonText || "";
  const proxyAuthEnabled = config?.proxyAuthEnabled || false;

  useEffect(() => {
    // Focus the username field when component mounts
    const usernameInput = document.getElementById("username");
    if (usernameInput) {
      usernameInput.focus();
    }

    // Check registration settings using public endpoint
    const fetchRegistrationStatus = async () => {
      try {
        const registrationResponse = await fetch("/api/auth/registration-status", {
          credentials: "include",
        });
        if (registrationResponse.ok) {
          const registrationData = await registrationResponse.json();
          setRegistrationEnabled(registrationData.enabled || false);
        }
      } catch (error) {
        console.error("Failed to fetch registration status:", error);
      }
    };

    if (multiUserMode) {
      fetchRegistrationStatus();
    }
  }, [multiUserMode]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError("");

    try {
      const response = await fetch("/api/auth/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ username, password }),
        credentials: "include",
      });

      if (response.ok) {
        onLogin();
      } else {
        const data = await response.json();
        setError(data.error ? t(data.error) : t("login.fail"));
      }
    } catch {
      setError(t("login.network_error"));
    } finally {
      setLoading(false);
    }
  };

  // Don't show the proxy auth message anymore since we support fallback
  // The form should always be available when proxy auth fails/isn't present

  const isSsoOnly = multiUserMode && oidcEnabled && oidcSsoOnly;

  // Don't render anything until we have config data to prevent flash
  if (multiUserMode && !config) {
    return null;
  }

  return (
    <div className={`bg-background ${isSsoOnly ? 'h-screen flex items-center justify-center px-4 -mt-32 md:-mt-20' : 'pt-0 pb-8 px-8'}`}>
      <div className={isSsoOnly ? 'w-full max-w-md' : ''}>
        <Card className={`${isSsoOnly ? 'w-[95%] mx-auto' : 'max-w-md mx-auto'} bg-card`}>
          {!isSsoOnly && (
            <CardHeader>
              <CardTitle className="text-xl">{t("login.title")}</CardTitle>
            </CardHeader>
          )}
          <CardContent className={isSsoOnly ? 'py-12 px-8' : ''}>
          {/* OIDC Login Button (multi-user mode only) */}
          {multiUserMode && oidcEnabled && (
            <div className={isSsoOnly ? 'flex flex-col items-center' : 'mb-6'}>
              <Button 
                type="button" 
                onClick={() => window.location.href = '/api/auth/oidc/login'}
                className="w-full"
                variant={isSsoOnly ? "default" : "outline"}
                disabled={loading}
              >
                {oidcButtonText || t("login.sso_button")}
              </Button>
              {/* Only show divider if not in SSO-only mode */}
              {!oidcSsoOnly && (
                <div className="relative my-4">
                  <div className="absolute inset-0 flex items-center">
                    <span className="w-full border-t" />
                  </div>
                  <div className="relative flex justify-center text-xs uppercase">
                    <span className="bg-card px-2 text-muted-foreground">
                      {t("login.or_continue_with")}
                    </span>
                  </div>
                </div>
              )}
            </div>
          )}
          
          {/* Username/Password form - hidden in SSO-only mode */}
          {!(multiUserMode && oidcEnabled && oidcSsoOnly) && (
            <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <Label htmlFor="username" className="mb-2 block">
                {t("login.username")}
              </Label>
              <Input
                id="username"
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                disabled={loading}
              />
            </div>
            <div>
              <Label htmlFor="password" className="mb-2 block">
                {t("login.password")}
              </Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                disabled={loading}
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div className="flex justify-between items-end">
              <div className="flex flex-col space-y-1">
                {multiUserMode && smtpConfigured && (
                  <Button 
                    type="button" 
                    variant="link" 
                    size="sm"
                    onClick={() => window.location.href = '/reset-password'}
                    disabled={loading}
                    className="text-sm text-muted-foreground hover:text-foreground p-0 h-auto justify-start"
                  >
                    {t("login.forgot_password")}
                  </Button>
                )}
                {multiUserMode && registrationEnabled && (
                  <Button 
                    type="button" 
                    variant="link" 
                    size="sm"
                    onClick={() => window.location.href = '/register'}
                    disabled={loading}
                    className="text-sm text-muted-foreground hover:text-foreground p-0 h-auto justify-start"
                  >
                    {t("login.register")}
                  </Button>
                )}
              </div>
              <Button type="submit" disabled={loading}>
                {loading ? t("login.signing_in") : t("login.button")}
              </Button>
            </div>
          </form>
          )}
        </CardContent>
      </Card>
      </div>
    </div>
  );
}
