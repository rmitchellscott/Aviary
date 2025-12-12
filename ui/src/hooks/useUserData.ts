import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@/components/AuthProvider";
import { useConfig } from "@/components/ConfigProvider";

interface UserData {
  id: string;
  username: string;
  email: string;
  is_admin: boolean;
  rmapi_host?: string;
  rmapi_paired?: boolean;
  default_rmdir: string;
  coverpage_setting: string;
  conflict_resolution: string;
  folder_depth_limit: number;
  folder_exclusion_list: string;
  page_resolution?: string;
  page_dpi?: number;
  conversion_output_format?: string;
  created_at: string;
  last_login?: string;
}

export function useUserData() {
  const { isAuthenticated, multiUserMode, user: authUser, refetchAuth } = useAuth();
  const { config, refetch: refetchConfig } = useConfig();
  const [user, setUser] = useState<UserData | null>(null);
  const [loading, setLoading] = useState(true);
  const [rmapiPaired, setRmapiPaired] = useState(false);
  const [rmapiHost, setRmapiHost] = useState("");

  const fetchUserData = useCallback(async () => {
    try {
      setLoading(true);
      
      // If not authenticated, clear everything
      if (!isAuthenticated) {
        setUser(null);
        setRmapiPaired(false);
        setRmapiHost("");
        return;
      }
      
      if (!config) {
        setLoading(false);
        return;
      }
      
      const isMultiUserMode = config.multiUserMode;
      
      if (isMultiUserMode) {
        // Multi-user mode: use auth user from AuthProvider
        if (authUser) {
          setUser(authUser);
          setRmapiPaired(!!authUser.rmapi_paired);
          
          // Use user's RMAPI_HOST setting, or empty string for official cloud
          setRmapiHost(authUser.rmapi_host || "");
        } else {
          setUser(null);
          setRmapiPaired(false);
          setRmapiHost(config.rmapi_host || "");
        }
      } else {
        // Single-user mode: get pairing status from config
        setUser(null); // No user object in single-user mode
        setRmapiPaired(!!config.rmapi_paired);
        setRmapiHost(config.rmapi_host || "");
      }
    } catch (error) {
      console.error("Failed to fetch user data:", error);
      setUser(null);
      setRmapiPaired(false);
      setRmapiHost("");
    } finally {
      setLoading(false);
    }
  }, [isAuthenticated, authUser, config]);

  useEffect(() => {
    if (config) {
      fetchUserData();
    }
  }, [fetchUserData, config]);

  // Listen for changes in localStorage to sync across components
  useEffect(() => {
    const handleStorageChange = () => {
      fetchUserData();
    };

    const handleLogout = () => {
      setUser(null);
      setRmapiPaired(false);
      setRmapiHost("");
    };

    window.addEventListener('storage', handleStorageChange);
    // Also listen for custom events on the same tab
    window.addEventListener('userDataChanged', handleStorageChange);
    window.addEventListener('logout', handleLogout);

    return () => {
      window.removeEventListener('storage', handleStorageChange);
      window.removeEventListener('userDataChanged', handleStorageChange);
      window.removeEventListener('logout', handleLogout);
    };
  }, [rmapiPaired, isAuthenticated]);

  const refetch = useCallback(() => {
    fetchUserData();
  }, [fetchUserData]);

  const updatePairingStatus = useCallback(async (paired: boolean) => {
    setRmapiPaired(paired);
    
    setTimeout(async () => {
      if (multiUserMode && refetchAuth) {
        await refetchAuth();
      } else if (refetchConfig) {
        await refetchConfig();
      }
      await fetchUserData();
      window.dispatchEvent(new CustomEvent('userDataChanged'));
    }, 100);
  }, [fetchUserData, multiUserMode, refetchAuth, refetchConfig]);

  return {
    user,
    loading,
    rmapiPaired,
    rmapiHost,
    refetch,
    updatePairingStatus,
  };
}