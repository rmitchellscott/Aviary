import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react';

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

interface ConfigContextType {
  config: Config | null;
  loading: boolean;
  error: string | null;
  refetch: () => Promise<void>;
}

const ConfigContext = createContext<ConfigContextType | undefined>(undefined);

export function useConfig() {
  const context = useContext(ConfigContext);
  if (!context) {
    throw new Error('useConfig must be used within a ConfigProvider');
  }
  return context;
}

interface ConfigProviderProps {
  children: ReactNode;
}

export function ConfigProvider({ children }: ConfigProviderProps) {
  const [config, setConfig] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchConfig = async () => {
    try {
      setLoading(true);
      setError(null);
      const response = await fetch('/api/config', {
        credentials: 'include',
      });

      if (response.ok) {
        const configData = await response.json();
        setConfig(configData);
      } else {
        setError('Failed to fetch configuration');
      }
    } catch (err) {
      console.error('Failed to fetch config:', err);
      setError('Failed to fetch configuration');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchConfig();
  }, []);

  return (
    <ConfigContext.Provider value={{ config, loading, error, refetch: fetchConfig }}>
      {children}
    </ConfigContext.Provider>
  );
}