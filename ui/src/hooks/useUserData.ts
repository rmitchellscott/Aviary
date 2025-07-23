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
  const { isAuthenticated } = useAuth();
  const [user, setUser] = useState<UserData | null>(null);
  const [loading, setLoading] = useState(true);
  const [rmapiPaired, setRmapiPaired] = useState(false);
  const [rmapiHost, setRmapiHost] = useState("");

  const fetchUserData = async () => {
    try {
      setLoading(true);
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
            // Fetch default RMAPI_HOST from system
            try {
              const configResponse = await fetch("/api/config", {
                credentials: "include",
              });
              if (configResponse.ok) {
                const config = await configResponse.json();
                setRmapiHost(config.rmapi_host || "");
              }
            } catch {
              setRmapiHost("");
            }
          }
        } else {
          setUser(null);
          setRmapiPaired(false);
          setRmapiHost("");
        }
      } else {
        setUser(null);
        setRmapiPaired(false);
        setRmapiHost("");
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
  }, []);

  // Listen for changes in localStorage to sync across components
  useEffect(() => {
    const handleStorageChange = () => {
      fetchUserData();
    };

    window.addEventListener('storage', handleStorageChange);
    // Also listen for custom events on the same tab
    window.addEventListener('userDataChanged', handleStorageChange);

    return () => {
      window.removeEventListener('storage', handleStorageChange);
      window.removeEventListener('userDataChanged', handleStorageChange);
    };
  }, []);

  const refetch = useCallback(() => {
    fetchUserData();
  }, []);

  const updatePairingStatus = useCallback((paired: boolean) => {
    setRmapiPaired(paired);
    // Trigger a refetch after a short delay to ensure API calls complete
    setTimeout(() => {
      fetchUserData();
      // Notify other components by triggering a custom event
      window.dispatchEvent(new CustomEvent('userDataChanged'));
    }, 100);
  }, []);

  return {
    user,
    loading,
    rmapiPaired,
    rmapiHost,
    refetch,
    updatePairingStatus,
  };
}