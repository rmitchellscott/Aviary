import { useState, useEffect } from "react";

interface Config {
  apiUrl: string;
  authEnabled: boolean;
  apiKeyEnabled: boolean;
  multiUserMode: boolean;
  defaultRmDir: string;
  rmapi_host: string;
  smtpConfigured: boolean;
  oidcEnabled: boolean;
  oidcSsoOnly: boolean;
  oidcButtonText: string;
  proxyAuthEnabled: boolean;
  oidcGroupBasedAdmin: boolean;
  rmapi_paired?: boolean;
}

export function useConfig() {
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchConfig = async () => {
    try {
      setLoading(true);
      const response = await fetch("/api/config", {
        credentials: "include",
      });

      if (response.ok) {
        const configData = await response.json();
        setConfig(configData);
      }
    } catch (error) {
      console.error("Failed to fetch config:", error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchConfig();
  }, []);

  return {
    config,
    loading,
    refetch: fetchConfig,
  };
}