import React, { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { PairingDialog } from "@/components/PairingDialog";
import { useUserData } from "@/hooks/useUserData";
import { useConfig } from "@/hooks/useConfig";
import { useFolderRefresh } from "@/hooks/useFolderRefresh";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Settings,
  Key,
  User,
  UserCog,
  Save,
  Plus,
  Copy,
  Eye,
  EyeOff,
  CheckCircle,
  XCircle,
  Clock,
  AlertTriangle,
} from "lucide-react";

interface APIKey {
  id: string;
  name: string;
  key_prefix: string;
  is_active: boolean;
  last_used?: string;
  expires_at?: string;
  created_at: string;
}

interface UserSettingsProps {
  isOpen: boolean;
  onClose: () => void;
}

export function UserSettings({ isOpen, onClose }: UserSettingsProps) {
  const { t } = useTranslation();
  const { user, loading: userDataLoading, rmapiPaired, rmapiHost, refetch, updatePairingStatus } = useUserData();
  const { config } = useConfig();
  const { triggerRefresh, refreshTrigger } = useFolderRefresh();

  // Device presets for image to PDF conversion
  const devicePresets = {
    remarkable_1_2: { resolution: "1404x1872", dpi: 226 },
    remarkable_paper_pro: { resolution: "1620x2160", dpi: 229 }
  };

  // Helper function to determine device preset from user settings
  const getDevicePresetFromUser = (pageResolution?: string, pageDPI?: number) => {
    if (!pageResolution && !pageDPI) {
      return "remarkable_1_2"; // Default
    }
    
    if (pageResolution === "1620x2160" && pageDPI === 229) {
      return "remarkable_paper_pro";
    }
    
    if (pageResolution === "1404x1872" && pageDPI === 226) {
      return "remarkable_1_2";
    }
    
    return "manual";
  };

  // Helper function to get actual values for API call
  const getPageSettingsForAPI = () => {
    if (devicePreset === "manual") {
      return {
        page_resolution: manualPageResolution || "",
        page_dpi: manualPageDPI ? parseFloat(manualPageDPI) : 0
      };
    } else {
      const preset = devicePresets[devicePreset as keyof typeof devicePresets];
      return {
        page_resolution: preset.resolution,
        page_dpi: preset.dpi
      };
    }
  };
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showCopiedText, setShowCopiedText] = useState(false);
  const [maxApiKeys, setMaxApiKeys] = useState<number>(10);

  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [userRmapiHost, setUserRmapiHost] = useState("");
  const [defaultRmdir, setDefaultRmdir] = useState("/");
  const [coverpageSetting, setCoverpageSetting] = useState("current");
  const [conflictResolution, setConflictResolution] = useState("abort");
  const [devicePreset, setDevicePreset] = useState("remarkable_1_2");
  const [manualPageResolution, setManualPageResolution] = useState("");
  const [manualPageDPI, setManualPageDPI] = useState("");
  const [folderDepthLimit, setFolderDepthLimit] = useState("");
  const [folderExclusionList, setFolderExclusionList] = useState("");
  
  // Original values for change tracking
  const [originalValues, setOriginalValues] = useState({
    username: "",
    email: "",
    userRmapiHost: "",
    defaultRmdir: "/",
    coverpageSetting: "current",
    conflictResolution: "abort",
    devicePreset: "remarkable_1_2",
    manualPageResolution: "",
    manualPageDPI: "",
    folderDepthLimit: "",
    folderExclusionList: ""
  });
  
  const [folders, setFolders] = useState<string[]>([]);
  const [foldersLoading, setFoldersLoading] = useState(false);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [deletePassword, setDeletePassword] = useState("");
  const [deleteConfirmation, setDeleteConfirmation] = useState("");

  const [newKeyName, setNewKeyName] = useState("");
  const [newKeyExpiry, setNewKeyExpiry] = useState("never");
  const [showNewKey, setShowNewKey] = useState<string | null>(null);

  const [pairingDialogOpen, setPairingDialogOpen] = useState(false);
  const [deleteAccountDialog, setDeleteAccountDialog] = useState(false);

  const [deleteKeyDialog, setDeleteKeyDialog] = useState<{
    isOpen: boolean;
    key: APIKey | null;
  }>({ isOpen: false, key: null });

  const [viewKey, setViewKey] = useState<APIKey | null>(null);

  const handleClose = () => {
    setError(null);
    setShowCopiedText(false);
    onClose();
  };

  useEffect(() => {
    if (isOpen) {
      fetchAPIKeys();
      if (user) {
        const email = user.email;
        const userRmapiHost = user.rmapi_host || "";
        const defaultRmdir = user.default_rmdir || "/";
        const coverpageSetting = user.coverpage_setting || "current";
        const conflictResolution = user.conflict_resolution || "abort";
        const folderDepthLimit = user.folder_depth_limit && user.folder_depth_limit > 0 ? user.folder_depth_limit.toString() : "";
        const folderExclusionList = user.folder_exclusion_list || "";
        const detectedPreset = getDevicePresetFromUser(user.page_resolution, user.page_dpi);
        const manualResolution = detectedPreset === "manual" ? (user.page_resolution || "") : "";
        const manualDPI = detectedPreset === "manual" ? (user.page_dpi?.toString() || "") : "";
        
        setUsername(user.username);
        setEmail(email);
        setUserRmapiHost(userRmapiHost);
        setDefaultRmdir(defaultRmdir);
        setCoverpageSetting(coverpageSetting);
        setConflictResolution(conflictResolution);
        setFolderDepthLimit(folderDepthLimit);
        setFolderExclusionList(folderExclusionList);
        setDevicePreset(detectedPreset);
        setManualPageResolution(manualResolution);
        setManualPageDPI(manualDPI);
      }
    }
  }, [isOpen, user]);

  // Update form fields when user data changes
  useEffect(() => {
    if (user) {
      const email = user.email;
      const userRmapiHost = user.rmapi_host || "";
      const defaultRmdir = user.default_rmdir || "/";
      const coverpageSetting = user.coverpage_setting || "current";
      const conflictResolution = user.conflict_resolution || "abort";
      const folderDepthLimit = user.folder_depth_limit && user.folder_depth_limit > 0 ? user.folder_depth_limit.toString() : "";
      const folderExclusionList = user.folder_exclusion_list || "";
      const detectedPreset = getDevicePresetFromUser(user.page_resolution, user.page_dpi);
      const manualResolution = detectedPreset === "manual" ? (user.page_resolution || "") : "";
      const manualDPI = detectedPreset === "manual" ? (user.page_dpi?.toString() || "") : "";
      
      setUsername(user.username);
      setEmail(email);
      setUserRmapiHost(userRmapiHost);
      setDefaultRmdir(defaultRmdir);
      setCoverpageSetting(coverpageSetting);
      setConflictResolution(conflictResolution);
      setFolderDepthLimit(folderDepthLimit);
      setFolderExclusionList(folderExclusionList);
      setDevicePreset(detectedPreset);
      setManualPageResolution(manualResolution);
      setManualPageDPI(manualDPI);
      
      setOriginalValues({
        username: user.username,
        email,
        userRmapiHost,
        defaultRmdir,
        coverpageSetting,
        conflictResolution,
        folderDepthLimit,
        folderExclusionList,
        devicePreset: detectedPreset,
        manualPageResolution: manualResolution,
        manualPageDPI: manualDPI
      });
    }
  }, [user]);

  // Fetch folders when user data is loaded and rmapi is paired
  useEffect(() => {
    if (isOpen && user && rmapiPaired) {
      fetchFolders();
    } else {
      // Clear folders if not paired
      setFolders([]);
      setFoldersLoading(false);
    }
  }, [isOpen, user, rmapiPaired, refreshTrigger]);

  // Listen for logout event to clear sensitive state
  useEffect(() => {
    const handleLogout = () => {
      // Clear all sensitive state
      setApiKeys([]);
      setError(null);
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setDeletePassword("");
      setDeleteConfirmation("");
      setNewKeyName("");
      setNewKeyExpiry("");
      setShowNewKey(null);
      setViewKey(null);
      
      // Close any open dialogs
      setPairingDialogOpen(false);
      setDeleteAccountDialog(false);
      setDeleteKeyDialog({ isOpen: false, key: null });
      
      // Reset form state to defaults
      setUsername("");
      setEmail("");
      setUserRmapiHost("");
      setDefaultRmdir("/");
      setCoverpageSetting("current");
      setConflictResolution("abort");
      setDevicePreset("remarkable_1_2");
      setManualPageResolution("");
      setManualPageDPI("");
      setFolderDepthLimit("");
      setFolderExclusionList("");
      
      // Reset original values
      setOriginalValues({
        username: "",
        email: "",
        userRmapiHost: "",
        defaultRmdir: "/",
        coverpageSetting: "current",
        conflictResolution: "abort",
        devicePreset: "remarkable_1_2",
        manualPageResolution: "",
        manualPageDPI: "",
        folderDepthLimit: "",
        folderExclusionList: ""
      });
      
      setFolders([]);
    };

    window.addEventListener('logout', handleLogout);

    return () => {
      window.removeEventListener('logout', handleLogout);
    };
  }, []);

  const fetchAPIKeys = async () => {
    try {
      const response = await fetch("/api/api-keys", {
        credentials: "include",
      });

      if (response.ok) {
        const data = await response.json();
        // Handle both old and new response formats for backward compatibility
        if (Array.isArray(data)) {
          // Old format: direct array of API keys
          setApiKeys(data);
        } else {
          // New format: object with api_keys, max_api_keys, etc.
          setApiKeys(data.api_keys || []);
          if (data.max_api_keys) {
            setMaxApiKeys(data.max_api_keys);
          }
        }
      }
    } catch (error) {
      console.error("Failed to fetch API keys:", error);
    }
  };


  const fetchFolders = async () => {
    try {
      setFoldersLoading(true);
      const response = await fetch("/api/folders?refresh=1", {
        credentials: "include",
      });

      if (response.ok) {
        const res = await response.json();
        if (Array.isArray(res.folders)) {
          const cleaned = res.folders
            .map((f: string) => f.replace(/^\//, ""))
            .filter((f: string) => f !== "");
          setFolders(cleaned);
        }
      }
    } catch (error) {
      console.error("Failed to fetch folders:", error);
    } finally {
      setFoldersLoading(false);
    }
  };

  // Check if there are unsaved changes
  const hasChanges = () => {
    return (
      (!config?.oidcEnabled && username !== originalValues.username) ||
      (!config?.oidcEnabled && email !== originalValues.email) ||
      userRmapiHost !== originalValues.userRmapiHost ||
      defaultRmdir !== originalValues.defaultRmdir ||
      coverpageSetting !== originalValues.coverpageSetting ||
      conflictResolution !== originalValues.conflictResolution ||
      folderDepthLimit !== originalValues.folderDepthLimit ||
      folderExclusionList !== originalValues.folderExclusionList ||
      devicePreset !== originalValues.devicePreset ||
      manualPageResolution !== originalValues.manualPageResolution ||
      manualPageDPI !== originalValues.manualPageDPI
    );
  };

  const updateProfile = async () => {
    try {
      setSaving(true);
      setError(null);

      const pageSettings = getPageSettingsForAPI();
      const response = await fetch("/api/profile", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          ...(config?.oidcEnabled ? {} : { username, email }),
          rmapi_host: userRmapiHost,
          default_rmdir: defaultRmdir,
          coverpage_setting: coverpageSetting,
          conflict_resolution: conflictResolution,
          folder_depth_limit: folderDepthLimit === "" ? 0 : parseInt(folderDepthLimit),
          folder_exclusion_list: folderExclusionList,
          ...pageSettings,
        }),
      });

      if (response.ok) {
        const folderSettingsChanged = 
          folderDepthLimit !== originalValues.folderDepthLimit ||
          folderExclusionList !== originalValues.folderExclusionList;

        // Update original values to reflect the saved state
        const detectedPreset = devicePreset;
        const manualResolution = detectedPreset === "manual" ? manualPageResolution : "";
        const manualDPI = detectedPreset === "manual" ? manualPageDPI : "";
        
        setOriginalValues({
          username,
          email,
          userRmapiHost,
          defaultRmdir,
          coverpageSetting,
          conflictResolution,
          folderDepthLimit,
          folderExclusionList,
          devicePreset: detectedPreset,
          manualPageResolution: manualResolution,
          manualPageDPI: manualDPI
        });

        // Trigger folder refresh if folder settings changed
        if (folderSettingsChanged && rmapiPaired) {
          triggerRefresh();
        }
        
        // Refetch user data to ensure we have the latest from server
        // This happens after updating originalValues to prevent flash
        setTimeout(() => {
          refetch();
        }, 100);
      } else {
        const errorData = await response.json();
        setError(errorData.error || "Failed to update profile");
      }
    } catch (error) {
      setError("Failed to update profile");
    } finally {
      setSaving(false);
    }
  };

  const handlePairingSuccess = () => {
    updatePairingStatus(true);
  };

  const disconnectRmapi = async () => {
    try {
      setSaving(true);
      setError(null);
      await fetch("/api/profile/disconnect", {
        method: "POST",
        credentials: "include",
      });
      updatePairingStatus(false);
    } catch {
      setError("Failed to disconnect");
    } finally {
      setSaving(false);
    }
  };

  const updatePassword = async () => {
    if (newPassword !== confirmPassword) {
      setError("Passwords do not match");
      return;
    }

    try {
      setSaving(true);
      setError(null);

      const response = await fetch("/api/profile/password", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      });

      if (response.ok) {
        setCurrentPassword("");
        setNewPassword("");
        setConfirmPassword("");
      } else {
        const errorData = await response.json();
        setError(errorData.error || "Failed to update password");
      }
    } catch (error) {
      setError("Failed to update password");
    } finally {
      setSaving(false);
    }
  };

  const createAPIKey = async () => {
    try {
      setSaving(true);
      setError(null);

      const body: any = { name: newKeyName };
      if (newKeyExpiry && newKeyExpiry !== "" && newKeyExpiry !== "never") {
        let expiryDate: Date;
        const now = new Date();
        switch (newKeyExpiry) {
          case "1week":
            expiryDate = new Date(now.getTime() + 7 * 24 * 60 * 60 * 1000);
            break;
          case "1month":
            expiryDate = new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000);
            break;
          case "3months":
            expiryDate = new Date(now.getTime() + 90 * 24 * 60 * 60 * 1000);
            break;
          case "1year":
            expiryDate = new Date(now.getTime() + 365 * 24 * 60 * 60 * 1000);
            break;
          default:
            expiryDate = new Date(newKeyExpiry); // fallback for date strings
        }
        body.expires_at = Math.floor(expiryDate.getTime() / 1000);
      }

      const response = await fetch("/api/api-keys", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify(body),
      });

      if (response.ok) {
        const newKey = await response.json();
        setShowNewKey(newKey.api_key);
        setNewKeyName("");
        setNewKeyExpiry("never");
        await fetchAPIKeys();
      } else {
        const errorData = await response.json();
        const errorMessage = errorData.error || "Failed to create API key";
        
        // Check for specific API key limit error and use i18n message
        if (errorMessage === "Maximum number of API keys reached" || 
            errorMessage.includes("maximum number of API keys") || 
            errorMessage.includes("reached")) {
          setError(t('settings.messages.api_key_limit_reached', { maxKeys: maxApiKeys }));
        } else {
          setError(errorMessage);
        }
      }
    } catch (error) {
      setError("Failed to create API key");
    } finally {
      setSaving(false);
    }
  };

  const openDeleteKeyDialog = (key: APIKey) => {
    setDeleteKeyDialog({ isOpen: true, key });
  };

  const closeDeleteKeyDialog = () => {
    setDeleteKeyDialog({ isOpen: false, key: null });
  };

  const confirmDeleteAPIKey = async () => {
    if (!deleteKeyDialog.key) return;

    try {
      const response = await fetch(`/api/api-keys/${deleteKeyDialog.key.id}`, {
        method: "DELETE",
        credentials: "include",
      });

      if (response.ok) {
        await fetchAPIKeys();
        closeDeleteKeyDialog();
      } else {
        const errorData = await response.json();
        setError(errorData.error || "Failed to delete API key");
      }
    } catch (error) {
      setError("Failed to delete API key");
    }
  };

  const openDeleteAccountDialog = () => {
    setDeletePassword("");
    setDeleteConfirmation("");
    setDeleteAccountDialog(true);
  };

  const closeDeleteAccountDialog = () => {
    setDeleteAccountDialog(false);
    setDeletePassword("");
    setDeleteConfirmation("");
  };

  const confirmDeleteAccount = async () => {
    try {
      setSaving(true);
      setError(null);

      const response = await fetch("/api/profile", {
        method: "DELETE",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          current_password: deletePassword,
          confirmation: deleteConfirmation,
        }),
      });

      if (response.ok) {
        // Account deleted successfully, redirect to login
        window.location.href = "/login";
      } else {
        const errorData = await response.json();
        setError(errorData.error || "Failed to delete account");
      }
    } catch (error) {
      setError("Failed to delete account");
    } finally {
      setSaving(false);
      setDeleteAccountDialog(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setShowCopiedText(true);
    setTimeout(() => setShowCopiedText(false), 3000);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString();
  };

  const getKeyStatus = (key: APIKey) => {
    if (!key.is_active) return "inactive";
    if (key.expires_at && new Date(key.expires_at) < new Date())
      return "expired";
    return "active";
  };

  const canDeleteAccount = deletePassword && deleteConfirmation === t('settings.placeholders.delete_confirm');

  if (userDataLoading) {
    return (
      <Dialog open={isOpen} onOpenChange={handleClose}>
        <DialogContent className="max-w-4xl">
          <div className="flex items-center justify-center p-8">{t('settings.loading_states.loading')}</div>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <>
      <Dialog open={isOpen} onOpenChange={handleClose}>
        <DialogContent className="max-w-7xl max-h-[85vh] overflow-y-auto sm:max-w-7xl sm:max-h-[90vh]">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
            <Settings className="h-5 w-5" />
            {t("settings.title")}
            </DialogTitle>
            <DialogDescription>
            {t("settings.description")}
            </DialogDescription>
          </DialogHeader>

          {error && (
            <div className="bg-destructive/10 border border-destructive/20 rounded-md p-3 text-destructive">
              {error}
            </div>
          )}


          <Tabs defaultValue="profile" className="w-full h-[660px] flex flex-col">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="profile" className="flex items-center gap-2">
                <User className="h-4 w-4" />
                <span className="hidden sm:inline">{t("settings.tabs.profile")}</span>
              </TabsTrigger>
              <TabsTrigger value="account" className="flex items-center gap-2">
                <UserCog className="h-4 w-4" />
                <span className="hidden sm:inline">{t("settings.tabs.account")}</span>
              </TabsTrigger>
              <TabsTrigger value="api-keys" className="flex items-center gap-2">
                <Key className="h-4 w-4" />
                <span className="hidden sm:inline">{t("settings.tabs.api_keys")}</span>
              </TabsTrigger>
            </TabsList>

            <TabsContent value="profile" className="flex-1 overflow-y-auto">
              <Card>
                <CardHeader>
                  <CardTitle>{t("settings.cards.profile_information")}</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <div>
                      <Label htmlFor="username">{t("settings.labels.username")}</Label>
                      {config?.oidcEnabled ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Input
                              id="username"
                              value={username}
                              readOnly
                              className="mt-2 bg-muted text-muted-foreground cursor-default"
                            />
                          </TooltipTrigger>
                          <TooltipContent>
                            {t("admin.tooltips.oidc_managed")}
                          </TooltipContent>
                        </Tooltip>
                      ) : (
                        <Input
                          id="username"
                          value={username}
                          disabled
                          className="mt-2"
                        />
                      )}
                    </div>
                    <div>
                      <Label htmlFor="email">{t("settings.labels.email")}</Label>
                      {config?.oidcEnabled ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Input
                              id="email"
                              type="email"
                              value={email}
                              readOnly
                              className="mt-2 bg-muted text-muted-foreground cursor-default"
                            />
                          </TooltipTrigger>
                          <TooltipContent>
                            {t("admin.tooltips.oidc_managed")}
                          </TooltipContent>
                        </Tooltip>
                      ) : (
                        <Input
                          id="email"
                          type="email"
                          value={email}
                          onChange={(e) => setEmail(e.target.value)}
                          className="mt-2"
                        />
                      )}
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                      <Label htmlFor="rmapi-host">{t("settings.labels.rmapi_host")}</Label>
                      <Input
                        id="rmapi-host"
                        value={userRmapiHost}
                        onChange={(e) => setUserRmapiHost(e.target.value)}
                        placeholder={t('settings.placeholders.cloud_default')}
                        className="mt-2"
                      />
                    </div>
                    <div className="flex items-end">
                      {rmapiPaired ? (
                        <Button
                          variant="outline"
                          onClick={disconnectRmapi}
                          disabled={saving}
                        >
                          {t("settings.actions.unpair")}
                        </Button>
                      ) : (
                        <Button
                          onClick={() => setPairingDialogOpen(true)}
                          disabled={saving}
                        >
                          {t("settings.actions.pair")}
                        </Button>
                      )}
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mt-8 mb-8">
                    <div>
                      <Label htmlFor="default-rmdir">{t("settings.labels.default_directory")}</Label>
                      <Select 
                        value={defaultRmdir} 
                        onValueChange={setDefaultRmdir}
                        disabled={!rmapiPaired}
                      >
                        <SelectTrigger id="default-rmdir" className="mt-2">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="/">
                            /
                          </SelectItem>
                          {!rmapiPaired && (
                            <SelectItem value="not-paired" disabled>
                              {t("settings.messages.pair_to_load_folders")}
                            </SelectItem>
                          )}
                          {rmapiPaired && foldersLoading && (
                            <SelectItem value="loading" disabled>
                              {t('settings.loading_states.loading')}
                            </SelectItem>
                          )}
                          {rmapiPaired && folders.map((f) => (
                            <SelectItem key={f} value={f}>
                              {f}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <p className="text-sm text-muted-foreground mt-1">
                        {!rmapiPaired ? t("settings.messages.pair_help") : t("settings.help.default_directory")}
                      </p>
                    </div>

                    <div>
                      <Label htmlFor="folder-depth-limit">{t("settings.labels.folder_depth_limit")}</Label>
                      <Input
                        id="folder-depth-limit"
                        type="number"
                        min="0"
                        value={folderDepthLimit}
                        onChange={(e) => setFolderDepthLimit(e.target.value)}
                        placeholder={t('settings.placeholders.folder_depth_limit')}
                        className="mt-2"
                      />
                      <p className="text-sm text-muted-foreground mt-1">
                        {t("settings.help.folder_depth_limit")}
                      </p>
                    </div>

                    <div>
                      <Label htmlFor="folder-exclusion-list">{t("settings.labels.folder_exclusion_list")}</Label>
                      <Input
                        id="folder-exclusion-list"
                        value={folderExclusionList}
                        onChange={(e) => setFolderExclusionList(e.target.value)}
                        placeholder={t('settings.placeholders.folder_exclusion_list')}
                        className="mt-2"
                      />
                      <p className="text-sm text-muted-foreground mt-1">
                        {t("settings.help.folder_exclusion_list")}
                      </p>
                    </div>

                    <div>
                      <Label htmlFor="coverpage-setting">{t("settings.labels.cover_page")}</Label>
                      <Select 
                        value={coverpageSetting} 
                        onValueChange={setCoverpageSetting}
                      >
                        <SelectTrigger id="coverpage-setting" className="mt-2">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="current">
                            {t("settings.options.cover_current")}
                          </SelectItem>
                          <SelectItem value="first">
                            {t("settings.options.cover_first")}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                      <p className="text-sm text-muted-foreground mt-1">
                        {t("settings.help.cover_page")}
                      </p>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                    <div>
                      <Label htmlFor="conflict-resolution">{t("settings.labels.conflict_resolution")}</Label>
                      <Select 
                        value={conflictResolution} 
                        onValueChange={setConflictResolution}
                      >
                        <SelectTrigger id="conflict-resolution" className="mt-2">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="abort">
                            {t("settings.options.conflict_abort")}
                          </SelectItem>
                          <SelectItem value="overwrite">
                            {t("settings.options.conflict_overwrite")}
                          </SelectItem>
                          <SelectItem value="content_only">
                            {t("settings.options.conflict_content_only")}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                      <p className="text-sm text-muted-foreground mt-1">
                        {t("settings.help.conflict_resolution")}
                      </p>
                    </div>

                    <div>
                      <Label htmlFor="device-preset">{t("settings.labels.pdf_conversion_device")}</Label>
                      <Select 
                        value={devicePreset} 
                        onValueChange={setDevicePreset}
                      >
                        <SelectTrigger id="device-preset" className="mt-2">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="remarkable_1_2">
                            reMarkable 1 & 2
                          </SelectItem>
                          <SelectItem value="remarkable_paper_pro">
                            reMarkable Paper Pro
                          </SelectItem>
                          <SelectItem value="manual">
                            {t("settings.options.manual")}
                          </SelectItem>
                        </SelectContent>
                      </Select>
                      <p className="text-sm text-muted-foreground mt-1">
                        {t("settings.help.pdf_conversion_device")}
                      </p>
                    </div>
                    
                    {devicePreset === "manual" && (
                      <div className="md:col-span-2 mt-4 space-y-4 p-4 bg-muted/50 rounded-md border">
                        <div>
                          <Label htmlFor="manual-resolution">{t("settings.labels.page_resolution")}</Label>
                          <Input
                            id="manual-resolution"
                            value={manualPageResolution}
                            onChange={(e) => setManualPageResolution(e.target.value)}
                            placeholder={t('settings.placeholders.page_resolution')}
                            className="mt-2"
                          />
                          <p className="text-sm text-muted-foreground mt-1">
                            {t("settings.help.page_resolution")}
                          </p>
                        </div>
                        <div>
                          <Label htmlFor="manual-dpi">{t("settings.labels.page_dpi")}</Label>
                          <Input
                            id="manual-dpi"
                            type="number"
                            value={manualPageDPI}
                            onChange={(e) => setManualPageDPI(e.target.value)}
                            placeholder={t('settings.placeholders.page_dpi')}
                            className="mt-2"
                          />
                          <p className="text-sm text-muted-foreground mt-1">
                            {t("settings.help.page_dpi")}
                          </p>
                        </div>
                      </div>
                    )}
                  </div>

                  <div className="flex justify-end">
                    <Button onClick={updateProfile} disabled={saving || !hasChanges()}>
                      <Save className="h-4 w-4 mr-2" />
                      {saving ? t('settings.loading_states.saving') : t('settings.buttons.save_changes')}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="account" className="flex-1 overflow-y-auto">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                <Card>
                  <CardHeader>
                    <CardTitle>{t("settings.cards.change_password")}</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div>
                      <Label htmlFor="current-password">{t("settings.labels.current_password")}</Label>
                      <Input
                        id="current-password"
                        type="password"
                        value={currentPassword}
                        onChange={(e) => setCurrentPassword(e.target.value)}
                        className="mt-2"
                      />
                    </div>

                    <div>
                      <Label htmlFor="new-password">{t("settings.labels.new_password")}</Label>
                      <Input
                        id="new-password"
                        type="password"
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        className="mt-2"
                      />
                    </div>

                    <div>
                      <Label htmlFor="confirm-password">{t("settings.labels.confirm_new_password")}</Label>
                      <Input
                        id="confirm-password"
                        type="password"
                        value={confirmPassword}
                        onChange={(e) => setConfirmPassword(e.target.value)}
                        className="mt-2"
                      />
                    </div>

                    <div className="flex justify-end">
                      <Button
                        onClick={updatePassword}
                        disabled={
                          saving ||
                          !currentPassword ||
                          !newPassword ||
                          !confirmPassword
                        }
                      >
                        <Save className="h-4 w-4 mr-2" />
                        {saving ? t('settings.loading_states.updating') : t('settings.buttons.update_password')}
                      </Button>
                    </div>
                  </CardContent>
                </Card>

                <Card className="border-destructive/20 bg-destructive/10">
                  <CardHeader>
                    <CardTitle className="text-destructive flex items-center gap-2">
                      <AlertTriangle className="h-5 w-5" />
                      {t("settings.cards.danger_zone")}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="bg-card p-4 rounded border">
                      <h3 className="font-medium text-destructive mb-2">
                        {t("settings.labels.delete_account")}
                      </h3>
                      <p className="text-sm text-destructive/80 mb-4">
                        {t("settings.messages.delete_warning_intro")}
                      </p>
                      <ul className="text-sm text-destructive/80 list-disc list-inside space-y-1 mb-4">
                        <li>{t("settings.delete_warnings.documents")}</li>
                        <li>{t("settings.delete_warnings.api_keys")}</li>
                        <li>{t("settings.delete_warnings.profile")}</li>
                      </ul>
                      <div className="bg-muted/50 p-3 rounded-md border mb-4">
                        <p className="text-sm text-muted-foreground">
                          <strong>{t("user_delete.note_label")}</strong> {t("settings.messages.remarkable_unaffected")}
                        </p>
                      </div>
                      <div className="flex justify-end">
                        <Button
                          variant="destructive"
                          onClick={openDeleteAccountDialog}
                        >
                          {t('settings.buttons.delete_my_account')}
                        </Button>
                      </div>
                    </div>
                  </CardContent>
                </Card>
              </div>
            </TabsContent>

            <TabsContent value="api-keys" className="flex-1 overflow-y-auto">
              <div className="space-y-4">
                <Card>
                  <CardHeader>
                    <CardTitle>{t("settings.cards.create_api_key")}</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <Label htmlFor="key-name">{t("settings.labels.api_key_name")}</Label>
                        <Input
                          id="key-name"
                          value={newKeyName}
                          onChange={(e) => setNewKeyName(e.target.value)}
                          onKeyDown={(e) => {
                            if (e.key === 'Enter' && newKeyName && !saving && apiKeys.length < maxApiKeys) {
                              e.preventDefault();
                              createAPIKey();
                            }
                          }}
                          placeholder={t('settings.placeholders.api_key')}
                          className="mt-2"
                        />
                        {apiKeys.length < maxApiKeys ? (
                          <p className="text-sm text-muted-foreground mt-1 ml-1">
                            {t('settings.messages.api_keys_remaining', { 
                              remaining: maxApiKeys - apiKeys.length, 
                              maxKeys: maxApiKeys 
                            })}
                          </p>
                        ) : (
                          <p className="text-sm text-destructive mt-1 ml-1">
                            {t('settings.messages.api_key_limit_reached', { maxKeys: maxApiKeys })}
                          </p>
                        )}
                      </div>
                      <div>
                        <Label htmlFor="key-expiry">{t("settings.labels.expires")}</Label>
                        <Select
                          value={newKeyExpiry}
                          onValueChange={setNewKeyExpiry}
                        >
                          <SelectTrigger className="mt-2">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="never">{t('settings.never')}</SelectItem>
                            <SelectItem value="1week">{t("settings.expiry_options.one_week")}</SelectItem>
                            <SelectItem value="1month">{t("settings.expiry_options.one_month")}</SelectItem>
                            <SelectItem value="3months">{t("settings.expiry_options.three_months")}</SelectItem>
                            <SelectItem value="1year">{t("settings.expiry_options.one_year")}</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    </div>

                    <div className="flex justify-end">
                      <Button
                        onClick={createAPIKey}
                        disabled={saving || !newKeyName || apiKeys.length >= maxApiKeys}
                      >
                        <Plus className="h-4 w-4 mr-2" />
                        {t("settings.actions.create_api_key")}
                      </Button>
                    </div>
                  </CardContent>
                </Card>

                {showNewKey && (
                  <Card className="border-primary/20 bg-primary/10">
                    <CardHeader>
                      <CardTitle className="text-primary">
                        {t("settings.messages.api_key_created_title")}
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <p className="text-sm text-primary/80 mb-2">
                        {t("settings.messages.api_key_created_message")}
                      </p>
                      <div className="flex items-center gap-2 p-2 bg-card rounded border">
                        <code className="flex-1 font-mono text-sm">
                          {showNewKey}
                        </code>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => copyToClipboard(showNewKey)}
                        >
                          {showCopiedText ? t('settings.tooltips.api_key_copied') : <Copy className="h-4 w-4" />}
                        </Button>
                      </div>
                      <div className="flex justify-end mt-2">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setShowNewKey(null)}
                        >
                          {t("settings.actions.got_it")}
                        </Button>
                      </div>
                    </CardContent>
                  </Card>
                )}

                <Card>
                  <CardHeader>
                    <CardTitle>{t("settings.cards.your_api_keys")}</CardTitle>
                  </CardHeader>
                  <CardContent>
                    {apiKeys.length === 0 ? (
                      <p className="text-center text-muted-foreground py-4">
                        {t("settings.messages.no_api_keys")}
                      </p>
                    ) : (
                      <Table className="w-full table-fixed lg:table-auto">
                        <TableHeader>
                          <TableRow>
                          <TableHead>{t("settings.labels.api_key_name")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("settings.labels.key_preview")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("settings.labels.status")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("settings.labels.created")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("settings.labels.last_used")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("settings.labels.expires")}</TableHead>
                          <TableHead></TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {apiKeys.map((key) => {
                            const status = getKeyStatus(key);
                            return (
                              <TableRow key={key.id}>
                                <TableCell className="font-medium">
                                  {key.name}
                                </TableCell>
                                <TableCell className="hidden lg:table-cell">
                                  <code className="text-sm">
                                    {key.key_prefix}...
                                  </code>
                                </TableCell>
                                <TableCell className="hidden lg:table-cell text-center">
                                  <Badge
                                    variant={
                                      status === "active"
                                        ? "success"
                                        : status === "expired"
                                          ? "destructive"
                                          : "secondary"
                                    }
                                    className="min-w-16 max-w-32 justify-center text-center whitespace-nowrap"
                                  >
                                    {t(`settings.status.${status}`)}
                                  </Badge>
                                </TableCell>
                                <TableCell className="hidden lg:table-cell">
                                  {formatDate(key.created_at)}
                                </TableCell>
                                <TableCell className="hidden lg:table-cell">
                                  {key.last_used
                                    ? formatDate(key.last_used)
                                    : t('settings.never')}
                                </TableCell>
                                <TableCell className="hidden lg:table-cell">
                                  {key.expires_at
                                    ? formatDate(key.expires_at)
                                    : t('settings.never')}
                                </TableCell>
                                <TableCell>
                                  <div className="flex gap-2">
                                    <Button
                                      size="sm"
                                      variant="outline"
                                      className="lg:hidden"
                                      onClick={() => setViewKey(key)}
                                    >
                                      {t('settings.actions.details', 'Details')}
                                    </Button>
                                    <Button
                                      size="sm"
                                      variant="destructive"
                                      onClick={() => openDeleteKeyDialog(key)}
                                    >
                                      {t("admin.actions.delete")}
                                    </Button>
                                  </div>
                                </TableCell>
                              </TableRow>
                            );
                          })}
                        </TableBody>
                      </Table>
                    )}
                  </CardContent>
                </Card>
              </div>
            </TabsContent>
          </Tabs>
        </DialogContent>
      </Dialog>

      {/* Pairing Dialog */}
      <PairingDialog
        isOpen={pairingDialogOpen}
        onClose={() => setPairingDialogOpen(false)}
        onPairingSuccess={handlePairingSuccess}
        rmapiHost={rmapiHost}
      />

      {/* Delete Account Dialog */}
      <AlertDialog open={deleteAccountDialog} onOpenChange={closeDeleteAccountDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-destructive" />
              Delete Account
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("settings.dialogs.delete_account_confirmation")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-4">
            <div>
              <Label htmlFor="delete-password">Current Password</Label>
              <Input
                id="delete-password"
                type="password"
                value={deletePassword}
                onChange={(e) => setDeletePassword(e.target.value)}
                placeholder={t('settings.placeholders.current_password')}
                className="mt-1"
              />
            </div>
            <div>
              <Label htmlFor="delete-confirmation">{t("settings.dialogs.delete_account_type_confirm", {confirmText: t('settings.placeholders.delete_confirm')})}</Label>
              <Input
                id="delete-confirmation"
                value={deleteConfirmation}
                onChange={(e) => setDeleteConfirmation(e.target.value)}
                placeholder={t('settings.placeholders.delete_confirm')}
                className="mt-1"
              />
            </div>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={closeDeleteAccountDialog} disabled={saving}>
              {t("settings.actions.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDeleteAccount}
              disabled={saving || !canDeleteAccount}
              className="bg-destructive hover:bg-destructive/90 focus:ring-destructive"
            >
              {saving ? t('settings.loading_states.deleting') : t('settings.buttons.delete_my_account')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Delete API Key Dialog */}
      <AlertDialog
        open={deleteKeyDialog.isOpen}
        onOpenChange={closeDeleteKeyDialog}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-destructive" />
              {t("settings.dialogs.delete_api_key_title")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("settings.dialogs.delete_api_key_message")}{" "}
              <strong>"{deleteKeyDialog.key?.name}"</strong>?
              <br />
              <br />
              {t("settings.dialogs.delete_api_key_consequences_title")}
              <ul className="list-disc list-inside mt-2 space-y-1">
                <li>{t("settings.dialogs.delete_api_key_consequences.revoke")}</li>
                <li>{t("settings.dialogs.delete_api_key_consequences.stop_apps")}</li>
                <li>{t("settings.dialogs.delete_api_key_consequences.remove")}</li>
              </ul>
              <br />
              <strong className="text-destructive">
                {t("settings.dialogs.cannot_undo")}
              </strong>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={closeDeleteKeyDialog}>
              {t("settings.actions.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction 
              onClick={confirmDeleteAPIKey}
              className="bg-destructive hover:bg-destructive/90 focus:ring-destructive"
            >
              {t("settings.actions.delete_api_key")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* API Key Details Dialog */}
      <Dialog open={!!viewKey} onOpenChange={() => setViewKey(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{viewKey?.name}</DialogTitle>
          </DialogHeader>
          {viewKey && (
            <div className="space-y-2 text-sm">
              <p>
                <strong>{t('settings.labels.key_preview')}:</strong>{' '}
                <code>{viewKey.key_prefix}...</code>
              </p>
              <p>
                <strong>{t('settings.labels.status')}:</strong>{' '}
                {getKeyStatus(viewKey)}
              </p>
              <p>
                <strong>{t('settings.labels.created')}:</strong>{' '}
                {formatDate(viewKey.created_at)}
              </p>
              <p>
                <strong>{t('settings.labels.last_used')}:</strong>{' '}
                {viewKey.last_used ? formatDate(viewKey.last_used) : t('settings.never')}
              </p>
              <p>
                <strong>{t('settings.labels.expires')}:</strong>{' '}
                {viewKey.expires_at ? formatDate(viewKey.expires_at) : t('settings.never')}
              </p>
            </div>
          )}
        </DialogContent>
      </Dialog>
    </>
  );
}
