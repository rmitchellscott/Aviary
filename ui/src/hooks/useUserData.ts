import { useState, useEffect, useCallback } from "react";
import { useAuth } from "@/components/AuthProvider";

interface UserData {
  id: string;
  username: string;
  email: string;
  is_admin: boolean;
  rmapi_host?: string;
  rmapi_paired?: boolean;
  default_rmdir: string;
  coverpage_setting: string;
  created_at: string;
  last_login?: string;
}

export function useUserData() {
  const { isAuthenticated, multiUserMode } = useAuth();
  const [user, setUser] = useState<UserData | null>(null);
  const [loading, setLoading] = useState(true);
  const [rmapiPaired, setRmapiPaired] = useState(false);
  const [rmapiHost, setRmapiHost] = useState("");

  const fetchUserData = async () => {
    try {
      setLoading(true);
      
      // If not authenticated, clear everything
      if (!isAuthenticated) {
        setUser(null);
        setRmapiPaired(false);
        setRmapiHost("");
        return;
      }
      
      // First, get config to determine if we're in multi-user mode
      const configResponse = await fetch("/api/config", {
        credentials: "include",
      });
      
      let config: any = {};
      if (configResponse.ok) {
        config = await configResponse.json();
      }
      
      const isMultiUserMode = config.multiUserMode;
      
      if (isMultiUserMode) {
        // Multi-user mode: check auth status for user data and pairing
        const response = await fetch("/api/auth/check", {
          credentials: "include",
        });

        if (response.ok) {
          const authData = await response.json();
          if (authData.authenticated && authData.user) {
            setUser(authData.user);
            setRmapiPaired(!!authData.user.rmapi_paired);
            
            // Default to environment RMAPI_HOST if user hasn't set their own
            if (authData.user.rmapi_host) {
              setRmapiHost(authData.user.rmapi_host);
            } else {
              setRmapiHost(config.rmapi_host || "");
            }
          } else {
            setUser(null);
            setRmapiPaired(false);
            setRmapiHost(config.rmapi_host || "");
          }
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
  };

  useEffect(() => {
    fetchUserData();
  }, [isAuthenticated]); // Re-fetch when authentication status changes

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
  }, [rmapiPaired, isAuthenticated]);

  const updatePairingStatus = useCallback((paired: boolean) => {
    setRmapiPaired(paired);
    
    // Trigger a refetch to sync with backend state
    setTimeout(() => {
      fetchUserData();
      // Notify other components by triggering a custom event
      window.dispatchEvent(new CustomEvent('userDataChanged'));
    }, 50); // Reduced from 100ms to 50ms
  }, [isAuthenticated]);

  return {
    user,
    loading,
    rmapiPaired,
    rmapiHost,
    refetch,
    updatePairingStatus,
  };
}