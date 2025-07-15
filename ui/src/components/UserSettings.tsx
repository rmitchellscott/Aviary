import React, { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { PairingDialog } from "@/components/PairingDialog";
import { useUserData } from "@/hooks/useUserData";
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
import { Textarea } from "@/components/ui/textarea";
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
  Trash2,
  Copy,
  Eye,
  EyeOff,
  CheckCircle,
  XCircle,
  Clock,
  AlertTriangle,
  UserX,
} from "lucide-react";

interface User {
  id: string;
  username: string;
  email: string;
  is_admin: boolean;
  rmapi_host?: string;
  default_rmdir: string;
  created_at: string;
  last_login?: string;
}

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
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Profile form
  const [email, setEmail] = useState("");
  const [userRmapiHost, setUserRmapiHost] = useState("");
  const [defaultRmdir, setDefaultRmdir] = useState("/");
  
  // Folder cache
  const [folders, setFolders] = useState<string[]>([]);
  const [foldersLoading, setFoldersLoading] = useState(false);

  // Password form
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  // Account deletion form
  const [deletePassword, setDeletePassword] = useState("");
  const [deleteConfirmation, setDeleteConfirmation] = useState("");

  // API key form
  const [newKeyName, setNewKeyName] = useState("");
  const [newKeyExpiry, setNewKeyExpiry] = useState("");
  const [showNewKey, setShowNewKey] = useState<string | null>(null);

  // Dialog states
  const [pairingDialogOpen, setPairingDialogOpen] = useState(false);
  const [deleteAccountDialog, setDeleteAccountDialog] = useState(false);

  // Delete API key dialog
  const [deleteKeyDialog, setDeleteKeyDialog] = useState<{
    isOpen: boolean;
    key: APIKey | null;
  }>({ isOpen: false, key: null });

  useEffect(() => {
    if (isOpen) {
      fetchAPIKeys();
    }
  }, [isOpen]);

  // Update form fields when user data changes
  useEffect(() => {
    if (user) {
      setEmail(user.email);
      setUserRmapiHost(user.rmapi_host || "");
      setDefaultRmdir(user.default_rmdir || "/");
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
  }, [isOpen, user, rmapiPaired]);


  const fetchAPIKeys = async () => {
    try {
      const response = await fetch("/api/api-keys", {
        credentials: "include",
      });

      if (response.ok) {
        const keys = await response.json();
        setApiKeys(keys);
      }
    } catch (error) {
      console.error("Failed to fetch API keys:", error);
    }
  };

  const fetchFolders = async () => {
    try {
      setFoldersLoading(true);
      const response = await fetch("/api/folders", {
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

  const updateProfile = async () => {
    try {
      setSaving(true);
      setError(null);

      const response = await fetch("/api/profile", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          email,
          rmapi_host: userRmapiHost,
          default_rmdir: defaultRmdir,
        }),
      });

      if (response.ok) {
        refetch(); // Refresh user data
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
        setNewKeyExpiry("");
        await fetchAPIKeys();
      } else {
        const errorData = await response.json();
        setError(errorData.error || "Failed to create API key");
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

  const canDeleteAccount = deletePassword && deleteConfirmation === "DELETE MY ACCOUNT";

  if (userDataLoading) {
    return (
      <Dialog open={isOpen} onOpenChange={onClose}>
        <DialogContent className="max-w-4xl">
          <div className="flex items-center justify-center p-8">Loading...</div>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <>
      <Dialog open={isOpen} onOpenChange={onClose}>
        <DialogContent className="max-w-7xl max-h-[90vh] overflow-y-auto sm:max-w-7xl">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Settings className="h-5 w-5" />
              User Settings
            </DialogTitle>
            <DialogDescription>
              Manage your profile, API keys, and account preferences
            </DialogDescription>
          </DialogHeader>

          {error && (
            <div className="bg-destructive/10 border border-destructive/20 rounded-md p-3 text-destructive">
              {error}
            </div>
          )}

          <Tabs defaultValue="profile" className="w-full h-[600px] flex flex-col">
            <TabsList className="grid w-full grid-cols-3">
              <TabsTrigger value="profile" className="flex items-center gap-2">
                <User className="h-4 w-4" />
                Profile
              </TabsTrigger>
              <TabsTrigger value="account" className="flex items-center gap-2">
                <UserCog className="h-4 w-4" />
                Account
              </TabsTrigger>
              <TabsTrigger value="api-keys" className="flex items-center gap-2">
                <Key className="h-4 w-4" />
                API Keys
              </TabsTrigger>
            </TabsList>

            <TabsContent value="profile" className="flex-1 overflow-y-auto">
              <Card>
                <CardHeader>
                  <CardTitle>Profile Information</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <Label htmlFor="username">Username</Label>
                      <Input
                        id="username"
                        value={user?.username || ""}
                        disabled
                        className="mt-2"
                      />
                    </div>
                    <div>
                      <Label htmlFor="email">Email</Label>
                      <Input
                        id="email"
                        type="email"
                        value={email}
                        onChange={(e) => setEmail(e.target.value)}
                        className="mt-2"
                      />
                    </div>
                  </div>

                  <div>
                    <Label htmlFor="rmapi-host">reMarkable Host (optional)</Label>
                    <Input
                      id="rmapi-host"
                      value={userRmapiHost}
                      onChange={(e) => setUserRmapiHost(e.target.value)}
                      placeholder="Leave empty for reMarkable Cloud"
                      className="mt-2"
                    />
                    <div className="mt-2">
                      {rmapiPaired ? (
                        <Button
                          variant="outline"
                          onClick={disconnectRmapi}
                          disabled={saving}
                        >
                          Unpair
                        </Button>
                      ) : (
                        <Button
                          onClick={() => setPairingDialogOpen(true)}
                          disabled={saving}
                        >
                          Pair
                        </Button>
                      )}
                    </div>
                  </div>

                  <div>
                    <Label htmlFor="default-rmdir">Default Directory</Label>
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
                            Pair to load folders
                          </SelectItem>
                        )}
                        {rmapiPaired && foldersLoading && (
                          <SelectItem value="loading" disabled>
                            Loading...
                          </SelectItem>
                        )}
                        {rmapiPaired && folders.map((f) => (
                          <SelectItem key={f} value={f}>
                            {f}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    {!rmapiPaired && (
                      <p className="text-sm text-muted-foreground mt-1">
                        Pair with reMarkable cloud to select from existing folders
                      </p>
                    )}
                  </div>

                  <div className="flex justify-end">
                    <Button onClick={updateProfile} disabled={saving}>
                      <Save className="h-4 w-4 mr-2" />
                      {saving ? "Saving..." : "Save Changes"}
                    </Button>
                  </div>
                </CardContent>
              </Card>
            </TabsContent>

            <TabsContent value="account" className="flex-1 overflow-y-auto">
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
                <Card>
                  <CardHeader>
                    <CardTitle>Change Password</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div>
                      <Label htmlFor="current-password">Current Password</Label>
                      <Input
                        id="current-password"
                        type="password"
                        value={currentPassword}
                        onChange={(e) => setCurrentPassword(e.target.value)}
                        className="mt-2"
                      />
                    </div>

                    <div>
                      <Label htmlFor="new-password">New Password</Label>
                      <Input
                        id="new-password"
                        type="password"
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        className="mt-2"
                      />
                    </div>

                    <div>
                      <Label htmlFor="confirm-password">Confirm New Password</Label>
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
                        {saving ? "Updating..." : "Update Password"}
                      </Button>
                    </div>
                  </CardContent>
                </Card>

                <Card className="border-red-200 bg-red-50 dark:bg-red-950/20">
                  <CardHeader>
                    <CardTitle className="text-red-800 dark:text-red-200 flex items-center gap-2">
                      <AlertTriangle className="h-5 w-5" />
                      Danger Zone
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="bg-white dark:bg-gray-900 p-4 rounded border">
                      <h3 className="font-medium text-red-800 dark:text-red-200 mb-2">
                        Delete Account
                      </h3>
                      <p className="text-sm text-red-700 dark:text-red-300 mb-4">
                        Once you delete your account, there is no going back. This action is permanent and will remove all your data including:
                      </p>
                      <ul className="text-sm text-red-700 dark:text-red-300 list-disc list-inside space-y-1 mb-4">
                        <li>All archived documents and files</li>
                        <li>All API keys and their access</li>
                        <li>Your profile and account settings</li>
                      </ul>
                      <div className="bg-blue-50 dark:bg-blue-950/20 p-3 rounded border border-blue-200 dark:border-blue-800 mb-4">
                        <p className="text-sm text-blue-800 dark:text-blue-200">
                          <strong>Note:</strong> Your reMarkable Cloud account and device data will remain completely unaffected.
                        </p>
                      </div>
                      <div className="flex justify-end">
                        <Button
                          variant="destructive"
                          onClick={openDeleteAccountDialog}
                        >
                          <UserX className="h-4 w-4 mr-2" />
                          Delete My Account
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
                    <CardTitle>Create New API Key</CardTitle>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <Label htmlFor="key-name">Name</Label>
                        <Input
                          id="key-name"
                          value={newKeyName}
                          onChange={(e) => setNewKeyName(e.target.value)}
                          placeholder="My API Key"
                          className="mt-2"
                        />
                      </div>
                      <div>
                        <Label htmlFor="key-expiry">Expiry (optional)</Label>
                        <Select
                          value={newKeyExpiry}
                          onValueChange={setNewKeyExpiry}
                        >
                          <SelectTrigger className="mt-2">
                            <SelectValue placeholder="Never expires" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="never">Never</SelectItem>
                            <SelectItem value="1week">1 week</SelectItem>
                            <SelectItem value="1month">1 month</SelectItem>
                            <SelectItem value="3months">3 months</SelectItem>
                            <SelectItem value="1year">1 year</SelectItem>
                          </SelectContent>
                        </Select>
                      </div>
                    </div>

                    <div className="flex justify-end">
                      <Button
                        onClick={createAPIKey}
                        disabled={saving || !newKeyName}
                      >
                        <Plus className="h-4 w-4 mr-2" />
                        Create API Key
                      </Button>
                    </div>
                  </CardContent>
                </Card>

                {showNewKey && (
                  <Card className="border-green-200 bg-green-50 dark:bg-green-950/20">
                    <CardHeader>
                      <CardTitle className="text-green-800 dark:text-green-200">
                        New API Key Created
                      </CardTitle>
                    </CardHeader>
                    <CardContent>
                      <p className="text-sm text-green-700 dark:text-green-300 mb-2">
                        Copy this key now - it won't be shown again:
                      </p>
                      <div className="flex items-center gap-2 p-2 bg-white dark:bg-gray-900 rounded border">
                        <code className="flex-1 font-mono text-sm">
                          {showNewKey}
                        </code>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => copyToClipboard(showNewKey)}
                        >
                          <Copy className="h-4 w-4" />
                        </Button>
                      </div>
                      <div className="flex justify-end mt-2">
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => setShowNewKey(null)}
                        >
                          Got it
                        </Button>
                      </div>
                    </CardContent>
                  </Card>
                )}

                <Card>
                  <CardHeader>
                    <CardTitle>Your API Keys</CardTitle>
                  </CardHeader>
                  <CardContent>
                    {apiKeys.length === 0 ? (
                      <p className="text-center text-muted-foreground py-4">
                        No API keys created yet
                      </p>
                    ) : (
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Name</TableHead>
                            <TableHead>Key Preview</TableHead>
                            <TableHead>Status</TableHead>
                            <TableHead>Created</TableHead>
                            <TableHead>Last Used</TableHead>
                            <TableHead>Expires</TableHead>
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
                                <TableCell>
                                  <code className="text-sm">
                                    {key.key_prefix}...
                                  </code>
                                </TableCell>
                                <TableCell>
                                  <Badge
                                    variant={
                                      status === "active"
                                        ? "success"
                                        : status === "expired"
                                          ? "destructive"
                                          : "secondary"
                                    }
                                  >
                                    {status === "active" && (
                                      <CheckCircle className="h-3 w-3 mr-1" />
                                    )}
                                    {status === "expired" && (
                                      <XCircle className="h-3 w-3 mr-1" />
                                    )}
                                    {status === "inactive" && (
                                      <Clock className="h-3 w-3 mr-1" />
                                    )}
                                    {status}
                                  </Badge>
                                </TableCell>
                                <TableCell>
                                  {formatDate(key.created_at)}
                                </TableCell>
                                <TableCell>
                                  {key.last_used
                                    ? formatDate(key.last_used)
                                    : "Never"}
                                </TableCell>
                                <TableCell>
                                  {key.expires_at
                                    ? formatDate(key.expires_at)
                                    : "Never"}
                                </TableCell>
                                <TableCell>
                                  <Button
                                    size="sm"
                                    variant="destructive"
                                    onClick={() => openDeleteKeyDialog(key)}
                                  >
                                    <Trash2 className="h-4 w-4" />
                                  </Button>
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
              <AlertTriangle className="h-5 w-5 text-red-500" />
              Delete Account
            </AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to permanently delete your account? This action cannot be undone.
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
                placeholder="Enter your current password"
                className="mt-1"
              />
            </div>
            <div>
              <Label htmlFor="delete-confirmation">Type "DELETE MY ACCOUNT" to confirm</Label>
              <Input
                id="delete-confirmation"
                value={deleteConfirmation}
                onChange={(e) => setDeleteConfirmation(e.target.value)}
                placeholder="DELETE MY ACCOUNT"
                className="mt-1"
              />
            </div>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={closeDeleteAccountDialog} disabled={saving}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDeleteAccount}
              disabled={saving || !canDeleteAccount}
              className="bg-red-600 hover:bg-red-700 focus:ring-red-600"
            >
              {saving ? 'Deleting...' : 'Delete My Account'}
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
              <AlertTriangle className="h-5 w-5 text-red-500" />
              Delete API Key
            </AlertDialogTitle>
            <AlertDialogDescription>
              Are you sure you want to delete the API key{" "}
              <strong>"{deleteKeyDialog.key?.name}"</strong>?
              <br />
              <br />
              This action will:
              <ul className="list-disc list-inside mt-2 space-y-1">
                <li>Permanently revoke the API key</li>
                <li>Stop all applications using this key from working</li>
                <li>Remove the key from your account</li>
              </ul>
              <br />
              <strong className="text-red-600">
                This action cannot be undone.
              </strong>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={closeDeleteKeyDialog}>
              Cancel
            </AlertDialogCancel>
            <AlertDialogAction onClick={confirmDeleteAPIKey}>
              <Trash2 className="h-4 w-4 mr-2" />
              Delete API Key
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
