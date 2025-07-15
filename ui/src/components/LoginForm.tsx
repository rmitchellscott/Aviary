"use client";

import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useAuth } from "@/components/AuthProvider";

interface LoginFormProps {
  onLogin: () => void;
}

export function LoginForm({ onLogin }: LoginFormProps) {
  const { t } = useTranslation();
  const { multiUserMode } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [smtpConfigured, setSmtpConfigured] = useState(false);

  useEffect(() => {
    // Focus the username field when component mounts
    const usernameInput = document.getElementById("username");
    if (usernameInput) {
      usernameInput.focus();
    }

    // Fetch config to check SMTP status
    const fetchConfig = async () => {
      try {
        const response = await fetch("/api/config", {
          credentials: "include",
        });
        if (response.ok) {
          const config = await response.json();
          setSmtpConfigured(config.smtpConfigured || false);
        }
      } catch (error) {
        console.error("Failed to fetch config:", error);
      }
    };

    if (multiUserMode) {
      fetchConfig();
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

  return (
    <div className="bg-background pt-0 pb-8 px-8">
      <Card className="max-w-md mx-auto bg-card">
        <CardHeader>
          <CardTitle className="text-xl">{t("login.title")}</CardTitle>
        </CardHeader>
        <CardContent>
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
            {error && <p className="text-sm text-red-500">{error}</p>}
            <div className="flex justify-between items-center">
              {multiUserMode && smtpConfigured && (
                <Button 
                  type="button" 
                  variant="link" 
                  size="sm"
                  onClick={() => window.location.href = '/reset-password'}
                  disabled={loading}
                  className="text-sm text-muted-foreground hover:text-foreground"
                >
                  Forgot Password?
                </Button>
              )}
              <Button type="submit" disabled={loading} className="ml-auto">
                {loading ? t("login.signing_in") : t("login.button")}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
