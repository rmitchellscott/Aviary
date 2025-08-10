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
  
  // Add request deduplication
  const [configPromise, setConfigPromise] = useState<Promise<void> | null>(null);

  const fetchConfig = async () => {
    // If a request is already in progress, return the existing promise
    if (configPromise) {
      console.log('ConfigProvider: Reusing existing config promise');
      return configPromise;
    }

    console.log('ConfigProvider: Creating new config request');
    const promise = (async () => {
      try {
        setLoading(true);
        setError(null);
        console.log('ConfigProvider: Fetching /api/config');
        const response = await fetch('/api/config', {
          credentials: 'include',
        });

        if (response.ok) {
          const configData = await response.json();
          setConfig(configData);
          console.log('ConfigProvider: Config loaded successfully');
        } else {
          setError('Failed to fetch configuration');
        }
      } catch (err) {
        console.error('Failed to fetch config:', err);
        setError('Failed to fetch configuration');
      } finally {
        setLoading(false);
        setConfigPromise(null); // Clear promise after completion
        console.log('ConfigProvider: Promise cleared');
      }
    })();

    setConfigPromise(promise);
    return promise;
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