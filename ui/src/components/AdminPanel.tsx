import React, { useState, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { useUserData } from "@/hooks/useUserData";
import { UserDeleteDialog } from "@/components/UserDeleteDialog";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
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
import { Switch } from "@/components/ui/switch";
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
  Shield,
  Users,
  Key,
  Settings as SettingsIcon,
  Database,
  Plus,
  Trash2,
  Edit,
  CheckCircle,
  XCircle,
  Clock,
  Activity,
  Mail,
  Server,
  AlertTriangle,
} from "lucide-react";

interface User {
  id: string;
  username: string;
  email: string;
  is_admin: boolean;
  is_active: boolean;
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
  user_id: string;
  username: string;
}

interface SystemStatus {
  database: {
    total_users: number;
    active_users: number;
    admin_users: number;
    documents: number;
    active_sessions: number;
    api_keys: {
      total: number;
      active: number;
      expired: number;
      recently_used: number;
    };
  };
  smtp: {
    configured: boolean;
    status: string;
  };
  settings: {
    registration_enabled: string;
    max_api_keys_per_user: string;
    session_timeout_hours: string;
  };
  mode: string;
  dry_run: boolean;
}

interface AdminPanelProps {
  isOpen: boolean;
  onClose: () => void;
}

export function AdminPanel({ isOpen, onClose }: AdminPanelProps) {
  const { t } = useTranslation();
  const { user: currentUser } = useUserData();
  const [users, setUsers] = useState<User[]>([]);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [systemStatus, setSystemStatus] = useState<SystemStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  // User creation form
  const [newUsername, setNewUsername] = useState("");
  const [newEmail, setNewEmail] = useState("");
  const [newPassword, setNewPassword] = useState("");

  // Settings
  const [registrationEnabled, setRegistrationEnabled] = useState(false);
  const [maxApiKeys, setMaxApiKeys] = useState("10");
  const [sessionTimeout, setSessionTimeout] = useState("24");

  // Modal states
  const [resetPasswordDialog, setResetPasswordDialog] = useState<{
    isOpen: boolean;
    user: User | null;
  }>({ isOpen: false, user: null });
  const [deleteUserDialog, setDeleteUserDialog] = useState<{
    isOpen: boolean;
    user: User | null;
  }>({ isOpen: false, user: null });
  const [newPasswordValue, setNewPasswordValue] = useState("");
  const [deleting, setDeleting] = useState(false);

  // Database backup/restore
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [restoreConfirmDialog, setRestoreConfirmDialog] = useState<{
    isOpen: boolean;
    file: File | null;
  }>({ isOpen: false, file: null });
  const storageFileInputRef = useRef<HTMLInputElement>(null);
  const [storageRestoreDialog, setStorageRestoreDialog] = useState<{
    isOpen: boolean;
    file: File | null;
  }>({ isOpen: false, file: null });
  const [restoreMode, setRestoreMode] = useState<"skip" | "overwrite">("skip");

  useEffect(() => {
    if (isOpen) {
      fetchSystemStatus();
      fetchUsers();
      fetchAPIKeys();
    }
  }, [isOpen]);

  const fetchSystemStatus = async () => {
    try {
      const response = await fetch("/api/admin/status", {
        credentials: "include",
      });

      if (response.ok) {
        const status = await response.json();
        setSystemStatus(status);
        setRegistrationEnabled(status.settings.registration_enabled === "true");
        setMaxApiKeys(status.settings.max_api_keys_per_user);
        setSessionTimeout(status.settings.session_timeout_hours);
      }
    } catch (error) {
      console.error("Failed to fetch system status:", error);
    }
  };

  const fetchUsers = async () => {
    try {
      const response = await fetch("/api/users", {
        credentials: "include",
      });

      if (response.ok) {
        const data = await response.json();
        setUsers(data.users);
      }
    } catch (error) {
      console.error("Failed to fetch users:", error);
    }
  };

  const fetchAPIKeys = async () => {
    try {
      const response = await fetch("/api/admin/api-keys", {
        credentials: "include",
      });

      if (response.ok) {
        const data = await response.json();
        setApiKeys(data.api_keys);
      }
    } catch (error) {
      console.error("Failed to fetch API keys:", error);
    }
  };

  const createUser = async () => {
    try {
      setSaving(true);
      setError(null);

      const response = await fetch("/api/auth/register", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          username: newUsername,
          email: newEmail,
          password: newPassword,
        }),
      });

      if (response.ok) {
        setNewUsername("");
        setNewEmail("");
        setNewPassword("");
        await fetchUsers();
        await fetchSystemStatus();
      } else {
        const errorData = await response.json();
        setError(errorData.error || "Failed to create user");
      }
    } catch (error) {
      setError("Failed to create user");
    } finally {
      setSaving(false);
    }
  };

  const toggleUserStatus = async (userId: string, isActive: boolean) => {
    try {
      const endpoint = isActive ? "activate" : "deactivate";
      const response = await fetch(`/api/users/${userId}/${endpoint}`, {
        method: "POST",
        credentials: "include",
      });

      if (response.ok) {
        await fetchUsers();
      }
    } catch (error) {
      console.error("Failed to toggle user status:", error);
    }
  };

  const toggleAdminStatus = async (userId: string, makeAdmin: boolean) => {
    try {
      const endpoint = makeAdmin ? "promote" : "demote";
      const response = await fetch(`/api/users/${userId}/${endpoint}`, {
        method: "POST",
        credentials: "include",
      });

      if (response.ok) {
        await fetchUsers();
      }
    } catch (error) {
      console.error("Failed to toggle admin status:", error);
    }
  };

  const openDeleteUserDialog = (user: User) => {
    setDeleteUserDialog({ isOpen: true, user });
  };

  const closeDeleteUserDialog = () => {
    setDeleteUserDialog({ isOpen: false, user: null });
  };

  const confirmDeleteUser = async () => {
    if (!deleteUserDialog.user) return;

    try {
      setDeleting(true);
      const response = await fetch(`/api/users/${deleteUserDialog.user.id}`, {
        method: "DELETE",
        credentials: "include",
      });

      if (response.ok) {
        await fetchUsers();
        await fetchSystemStatus();
        closeDeleteUserDialog();
      } else {
        const errorData = await response.json();
        setError(errorData.error || "Failed to delete user");
      }
    } catch (error) {
      setError("Failed to delete user");
    } finally {
      setDeleting(false);
    }
  };

  const openResetPasswordDialog = (user: User) => {
    setResetPasswordDialog({ isOpen: true, user });
    setNewPasswordValue("");
  };

  const closeResetPasswordDialog = () => {
    setResetPasswordDialog({ isOpen: false, user: null });
    setNewPasswordValue("");
  };

  const handleClose = () => {
    setError(null);
    setSuccessMessage(null);
    onClose();
  };

  const confirmResetPassword = async () => {
    if (!resetPasswordDialog.user || !newPasswordValue) return;

    if (newPasswordValue.length < 8) {
      setError("Password must be at least 8 characters long");
      return;
    }

    try {
      setSaving(true);
      const response = await fetch(
        `/api/users/${resetPasswordDialog.user.id}/reset-password`,
        {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
          credentials: "include",
          body: JSON.stringify({ new_password: newPasswordValue }),
        },
      );

      if (response.ok) {
        closeResetPasswordDialog();
        setError(null);
      } else {
        const errorData = await response.json();
        setError(errorData.error || "Failed to reset password");
      }
    } catch (error) {
      setError("Failed to reset password");
    } finally {
      setSaving(false);
    }
  };

  const updateSystemSetting = async (key: string, value: string) => {
    try {
      const response = await fetch("/api/admin/settings", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({ key, value }),
      });

      if (response.ok) {
        await fetchSystemStatus();
      }
    } catch (error) {
      console.error("Failed to update setting:", error);
    }
  };

  const testSMTP = async () => {
    try {
      setSaving(true);
      setError(null);

      const response = await fetch("/api/admin/test-smtp", {
        method: "POST",
        credentials: "include",
      });

      const result = await response.json();
      if (response.ok) {
        setError(null);
        // Show success message
      } else {
        setError(result.error || "SMTP test failed");
      }
    } catch (error) {
      setError("Failed to test SMTP");
    } finally {
      setSaving(false);
    }
  };

  const handleBackupDatabase = async () => {
    try {
      setSaving(true);
      setError(null);

      const response = await fetch("/api/admin/backup", {
        method: "POST",
        credentials: "include",
      });

      if (response.ok) {
        // Get filename from Content-Disposition header or create default
        const contentDisposition = response.headers.get("Content-Disposition");
        let filename = "database_backup.db";
        if (contentDisposition) {
          const matches = contentDisposition.match(/filename=([^;]+)/);
          if (matches && matches[1]) {
            filename = matches[1].replace(/"/g, "");
          }
        }

        // Create blob and download
        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
      } else {
        const errorData = await response.json();
        setError(errorData.error || "Failed to create backup");
      }
    } catch (error) {
      setError("Failed to create backup");
    } finally {
      setSaving(false);
    }
  };

  const handleRestoreFileSelect = (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = event.target.files?.[0];
    if (file) {
      setRestoreConfirmDialog({ isOpen: true, file });
    }
    // Reset input value so same file can be selected again
    event.target.value = "";
  };

  const confirmDatabaseRestore = async () => {
    const file = restoreConfirmDialog.file;
    if (!file) return;

    try {
      setSaving(true);
      setError(null);

      const formData = new FormData();
      formData.append("backup_file", file);

      const response = await fetch("/api/admin/restore", {
        method: "POST",
        credentials: "include",
        body: formData,
      });

      const result = await response.json();
      if (response.ok) {
        setRestoreConfirmDialog({ isOpen: false, file: null });
        setError(null);
        setSuccessMessage(result.message || "Database restored successfully");
        // Refresh system status after restore
        await fetchSystemStatus();
        await fetchUsers();
        await fetchAPIKeys();
      } else {
        setError(result.error || "Failed to restore database");
      }
    } catch (error) {
      setError("Failed to restore database");
    } finally {
      setSaving(false);
    }
  };

  const closeRestoreConfirmDialog = () => {
    setRestoreConfirmDialog({ isOpen: false, file: null });
  };

  const handleBackupStorage = async () => {
    try {
      setSaving(true);
      setError(null);

      const response = await fetch("/api/admin/storage/backup", {
        method: "POST",
        credentials: "include",
      });

      if (response.ok) {
        const contentDisposition = response.headers.get("Content-Disposition");
        let filename = "storage_backup.tar.gz";
        if (contentDisposition) {
          const matches = contentDisposition.match(/filename=([^;]+)/);
          if (matches && matches[1]) {
            filename = matches[1].replace(/"/g, "");
          }
        }

        const blob = await response.blob();
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
      } else {
        const errorData = await response.json();
        setError(errorData.error || "Failed to create backup");
      }
    } catch (error) {
      setError("Failed to create backup");
    } finally {
      setSaving(false);
    }
  };

  const handleRestoreStorageSelect = (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = event.target.files?.[0];
    if (file) {
      setStorageRestoreDialog({ isOpen: true, file });
    }
    event.target.value = "";
  };

  const confirmStorageRestore = async () => {
    const file = storageRestoreDialog.file;
    if (!file) return;

    try {
      setSaving(true);
      setError(null);

      const formData = new FormData();
      formData.append("backup_file", file);
      formData.append("mode", restoreMode);

      const response = await fetch("/api/admin/storage/restore", {
        method: "POST",
        credentials: "include",
        body: formData,
      });

      const result = await response.json();
      if (response.ok) {
        setStorageRestoreDialog({ isOpen: false, file: null });
        setSuccessMessage(result.message || "Storage restored successfully");
      } else {
        setError(result.error || "Failed to restore storage");
      }
    } catch (error) {
      setError("Failed to restore storage");
    } finally {
      setSaving(false);
    }
  };

  const closeStorageRestoreDialog = () => {
    setStorageRestoreDialog({ isOpen: false, file: null });
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

  const getSMTPStatusColor = (status: string) => {
    switch (status) {
      case "configured_and_working":
        return "success";
      case "configured_but_failed":
        return "destructive";
      default:
        return "secondary";
    }
  };

  const getSMTPStatusText = (status: string) => {
    switch (status) {
      case "configured_and_working":
        return "Working";
      case "configured_but_failed":
        return "Failed";
      case "not_configured":
        return "Not configured";
      default:
        return "Unknown";
    }
  };

  if (!systemStatus) {
    return (
      <Dialog open={isOpen} onOpenChange={onClose}>
        <DialogContent className="max-w-6xl">
          <div className="flex items-center justify-center p-8">Loading...</div>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className="max-w-7xl max-h-[90vh] overflow-y-auto sm:max-w-7xl">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Shield className="h-5 w-5" />
            Admin Panel
          </DialogTitle>
          <DialogDescription>
            Manage users, system settings, and monitor system health
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-destructive/10 border border-destructive/20 rounded-md p-3 text-destructive">
            {error}
          </div>
        )}

        {successMessage && (
          <div className="bg-green-50 border border-green-200 rounded-md p-3 text-green-800">
            {successMessage}
          </div>
        )}

        <Tabs defaultValue="overview" className="w-full h-[600px] flex flex-col">
          <TabsList className="grid w-full grid-cols-5">
            <TabsTrigger value="overview" className="flex items-center gap-2">
              <Activity className="h-4 w-4" />
              Overview
            </TabsTrigger>
            <TabsTrigger value="users" className="flex items-center gap-2">
              <Users className="h-4 w-4" />
              Users
            </TabsTrigger>
            <TabsTrigger value="api-keys" className="flex items-center gap-2">
              <Key className="h-4 w-4" />
              API Keys
            </TabsTrigger>
            <TabsTrigger value="settings" className="flex items-center gap-2">
              <SettingsIcon className="h-4 w-4" />
              Settings
            </TabsTrigger>
            <TabsTrigger value="system" className="flex items-center gap-2">
              <Database className="h-4 w-4" />
              System
            </TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="flex-1 overflow-y-auto">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">
                    Total Users
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {systemStatus.database.total_users}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {systemStatus.database.active_users} active
                  </p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">
                    API Keys
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {systemStatus.database.api_keys.total}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {systemStatus.database.api_keys.active} active
                  </p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">
                    Documents
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {systemStatus.database.documents}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Total uploaded
                  </p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">
                    Active Sessions
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {systemStatus.database.active_sessions}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Current unexpired sessions
                  </p>
                </CardContent>
              </Card>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Mail className="h-5 w-5" />
                    SMTP Status
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center justify-between">
                    <Badge
                      variant={getSMTPStatusColor(systemStatus.smtp.status)}
                    >
                      {getSMTPStatusText(systemStatus.smtp.status)}
                    </Badge>
                    <Button size="sm" onClick={testSMTP} disabled={saving}>
                      Test SMTP
                    </Button>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Server className="h-5 w-5" />
                    System Mode
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="flex flex-col gap-2 items-start">
                    <Badge variant="success">Multi-user Mode</Badge>
                    {systemStatus.dry_run && (
                      <Badge variant="destructive">Dry Run Mode</Badge>
                    )}
                  </div>
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          <TabsContent value="users" className="flex-1 overflow-y-auto">
            <div className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>Create New User</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-3 gap-4">
                    <div>
                      <Label htmlFor="new-username">Username</Label>
                      <Input
                        id="new-username"
                        value={newUsername}
                        onChange={(e) => setNewUsername(e.target.value)}
                        placeholder="username"
                        className="mt-2"
                      />
                    </div>
                    <div>
                      <Label htmlFor="new-email">Email</Label>
                      <Input
                        id="new-email"
                        type="email"
                        value={newEmail}
                        onChange={(e) => setNewEmail(e.target.value)}
                        placeholder="user@example.com"
                        className="mt-2"
                      />
                    </div>
                    <div>
                      <Label htmlFor="new-password">Password</Label>
                      <Input
                        id="new-password"
                        type="password"
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        placeholder="password"
                        className="mt-2"
                      />
                    </div>
                  </div>

                  <div className="flex justify-end">
                    <Button
                      onClick={createUser}
                      disabled={
                        saving || !newUsername || !newEmail || !newPassword
                      }
                    >
                      <Plus className="h-4 w-4 mr-2" />
                      Create User
                    </Button>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Users ({users.length})</CardTitle>
                </CardHeader>
                <CardContent>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Username</TableHead>
                        <TableHead>Email</TableHead>
                        <TableHead>Role</TableHead>
                        <TableHead>Status</TableHead>
                        <TableHead>Created</TableHead>
                        <TableHead>Last Login</TableHead>
                        <TableHead>Actions</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {users.map((user) => (
                        <TableRow key={user.id}>
                          <TableCell className="font-medium">
                            {user.username}
                          </TableCell>
                          <TableCell>{user.email}</TableCell>
                          <TableCell>
                            <Badge
                              variant={user.is_admin ? "default" : "secondary"}
                              className="w-14 justify-center"
                            >
                              {user.is_admin ? "Admin" : "User"}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <Badge
                              variant={user.is_active ? "success" : "secondary"}
                              className="w-16 justify-center"
                            >
                              {user.is_active ? "Active" : "Inactive"}
                            </Badge>
                          </TableCell>
                          <TableCell>{formatDate(user.created_at)}</TableCell>
                          <TableCell>
                            {user.last_login
                              ? formatDate(user.last_login)
                              : "Never"}
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2">
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => openResetPasswordDialog(user)}
                              >
                                Reset Password
                              </Button>
                              {currentUser && user.id !== currentUser.id && (
                                <Button
                                  size="sm"
                                  variant="outline"
                                  onClick={() =>
                                    toggleAdminStatus(user.id, !user.is_admin)
                                  }
                                  className="w-24"
                                >
                                  {user.is_admin ? "Make User" : "Make Admin"}
                                </Button>
                              )}
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() =>
                                  toggleUserStatus(user.id, !user.is_active)
                                }
                                className="w-28"
                              >
                                {user.is_active ? "Deactivate" : "Activate"}
                              </Button>
                              <Button
                                size="sm"
                                variant="destructive"
                                onClick={() => openDeleteUserDialog(user)}
                              >
                                <Trash2 className="h-3 w-3 mr-1" />
                                Delete
                              </Button>
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          <TabsContent value="api-keys" className="flex-1 overflow-y-auto">
            <Card>
              <CardHeader>
                <CardTitle>All API Keys ({apiKeys.length})</CardTitle>
              </CardHeader>
              <CardContent>
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>User</TableHead>
                      <TableHead>Key Preview</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead>Last Used</TableHead>
                      <TableHead>Expires</TableHead>
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
                          <TableCell>{key.username}</TableCell>
                          <TableCell>
                            <code className="text-sm">{key.key_prefix}...</code>
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
                          <TableCell>{formatDate(key.created_at)}</TableCell>
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
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="settings" className="flex-1 overflow-y-auto">
            <div className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>User Management</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <Label htmlFor="registration-enabled">
                        Enable User Registration
                      </Label>
                      <p className="text-sm text-muted-foreground">
                        Allow admins to create new user accounts
                      </p>
                    </div>
                    <Switch
                      id="registration-enabled"
                      checked={registrationEnabled}
                      onCheckedChange={(checked) => {
                        setRegistrationEnabled(checked);
                        updateSystemSetting(
                          "registration_enabled",
                          checked.toString(),
                        );
                      }}
                    />
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>API Key Settings</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div>
                    <Label htmlFor="max-api-keys">
                      Maximum API Keys per User
                    </Label>
                    <Input
                      id="max-api-keys"
                      type="number"
                      value={maxApiKeys}
                      onChange={(e) => setMaxApiKeys(e.target.value)}
                      onBlur={() =>
                        updateSystemSetting("max_api_keys_per_user", maxApiKeys)
                      }
                      className="mt-2"
                    />
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Session Settings</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div>
                    <Label htmlFor="session-timeout">
                      Session Timeout (hours)
                    </Label>
                    <Input
                      id="session-timeout"
                      type="number"
                      value={sessionTimeout}
                      onChange={(e) => setSessionTimeout(e.target.value)}
                      onBlur={() =>
                        updateSystemSetting(
                          "session_timeout_hours",
                          sessionTimeout,
                        )
                      }
                      className="mt-2"
                    />
                  </div>
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          <TabsContent value="system" className="flex-1 overflow-y-auto">
            <div className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>Database Management</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <Button
                      variant="outline"
                      onClick={handleBackupDatabase}
                      disabled={saving}
                      className="w-full"
                    >
                      <Database className="h-4 w-4 mr-2" />
                      {saving ? "Creating Backup..." : "Backup Database"}
                    </Button>
                    <div className="w-full">
                      <input
                        type="file"
                        ref={fileInputRef}
                        onChange={handleRestoreFileSelect}
                        accept=".db,.sqlite,.sqlite3,.sql,.dump,.custom"
                        style={{ display: "none" }}
                      />
                      <Button
                        variant="outline"
                        onClick={() => fileInputRef.current?.click()}
                        disabled={saving}
                        className="w-full"
                      >
                        <Database className="h-4 w-4 mr-2" />
                        Restore Database
                      </Button>
                    </div>
                  </div>
                  <div className="text-sm text-muted-foreground space-y-2">
                    <p>
                      <strong>Backup:</strong> Downloads a complete backup of
                      the database
                    </p>
                    <p>
                      <strong>Restore:</strong> Replaces current database with
                      uploaded backup file
                    </p>
                    <p className="text-amber-600">
                      <AlertTriangle className="h-4 w-4 inline mr-1" />
                      Warning: Database restore will overwrite current database
                    </p>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Storage Management</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-2 gap-4">
                    <Button
                      variant="outline"
                      onClick={handleBackupStorage}
                      disabled={saving}
                      className="w-full"
                    >
                      <Server className="h-4 w-4 mr-2" />
                      {saving ? "Creating Backup..." : "Backup Storage"}
                    </Button>
                    <div className="w-full">
                      <input
                        type="file"
                        ref={storageFileInputRef}
                        onChange={handleRestoreStorageSelect}
                        accept=".tar.gz"
                        style={{ display: "none" }}
                      />
                      <Button
                        variant="outline"
                        onClick={() => storageFileInputRef.current?.click()}
                        disabled={saving}
                        className="w-full"
                      >
                        <Server className="h-4 w-4 mr-2" />
                        Restore Storage...
                      </Button>
                    </div>
                  </div>
                  <div className="text-sm text-muted-foreground space-y-2">
                    <p>
                      <strong>Backup:</strong> Downloads a backup of the users directory. Includes rmapi pairings and archived files.
                    </p>
                    <p>
                      <strong>Restore:</strong> Extracts uploaded archive into users directory
                    </p>
                  </div>
                </CardContent>
              </Card>

              {/* <Card>
                <CardHeader>
                  <CardTitle>Maintenance</CardTitle>
                </CardHeader>
                <CardContent>
                  <Button variant="outline">
                    <Trash2 className="h-4 w-4 mr-2" />
                    Cleanup Old Data
                  </Button>
                </CardContent>
              </Card> */}
            </div>
          </TabsContent>
        </Tabs>
      </DialogContent>

      {/* Password Reset Dialog */}
      <Dialog
        open={resetPasswordDialog.isOpen}
        onOpenChange={closeResetPasswordDialog}
      >
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Key className="h-5 w-5" />
              Reset Password
            </DialogTitle>
            <DialogDescription>
              Reset the password for user:{" "}
              <strong>{resetPasswordDialog.user?.username}</strong>
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div>
              <Label htmlFor="new-password">New Password</Label>
              <Input
                id="new-password"
                type="password"
                value={newPasswordValue}
                onChange={(e) => setNewPasswordValue(e.target.value)}
                placeholder="Enter new password (minimum 8 characters)"
                className="mt-2"
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={closeResetPasswordDialog}>
              Cancel
            </Button>
            <Button
              onClick={confirmResetPassword}
              disabled={saving || newPasswordValue.length < 8}
            >
              {saving ? "Resetting..." : "Reset Password"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete User Dialog */}
      <UserDeleteDialog
        isOpen={deleteUserDialog.isOpen}
        onClose={closeDeleteUserDialog}
        onConfirm={confirmDeleteUser}
        user={deleteUserDialog.user}
        isCurrentUser={false}
        loading={deleting}
      />

      {/* Database Restore Confirmation Dialog */}
      <AlertDialog
        open={restoreConfirmDialog.isOpen}
        onOpenChange={closeRestoreConfirmDialog}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-amber-500" />
              Confirm Database Restore
            </AlertDialogTitle>
            <AlertDialogDescription className="space-y-2">
              <p>
                You are about to restore the database from:{" "}
                <strong>{restoreConfirmDialog.file?.name}</strong>
              </p>
              <p className="text-destructive font-medium">
                This will permanently overwrite all current data including:
              </p>
              <ul className="list-disc list-inside text-sm space-y-1 ml-4">
                <li>All user accounts and settings</li>
                <li>All API keys and sessions</li>
                <li>All documents and folder cache</li>
                <li>All system settings</li>
              </ul>
              <p className="text-destructive font-medium">
                This action cannot be undone. Make sure you have a current
                backup before proceeding.
              </p>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={saving}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDatabaseRestore}
              disabled={saving}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {saving ? "Restoring..." : "Restore Database"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Storage Restore Confirmation Dialog */}
      <AlertDialog
        open={storageRestoreDialog.isOpen}
        onOpenChange={closeStorageRestoreDialog}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-amber-500" />
              Confirm Storage Restore
            </AlertDialogTitle>
            <AlertDialogDescription>
              You are about to restore the data directory from:{" "}
              <strong>{storageRestoreDialog.file?.name}</strong>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="space-y-2 py-2">
            <Label htmlFor="restore-mode">Conflict Resolution</Label>
            <Select
              value={restoreMode}
              onValueChange={(v) => setRestoreMode(v as "skip" | "overwrite")}
            >
              <SelectTrigger id="restore-mode">
                <SelectValue placeholder="Choose mode" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="skip">Skip Existing</SelectItem>
                <SelectItem value="overwrite">Overwrite</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={saving}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmStorageRestore}
              disabled={saving}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {saving ? "Restoring..." : "Restore Storage"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </Dialog>
  );
}
