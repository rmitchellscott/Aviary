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
import {
  Alert,
  AlertDescription,
} from "@/components/ui/alert";
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
  Shield,
  Users,
  Key,
  Settings as SettingsIcon,
  Database,
  Plus,
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
  rmapi_paired: boolean;
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
  auth: {
    oidc_enabled: boolean;
    proxy_auth_enabled: boolean;
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
  // SMTP test state
  const [smtpTestResult, setSmtpTestResult] = useState<'working' | 'failed' | null>(null);
  // User creation form
  const [newUsername, setNewUsername] = useState("");
  const [newEmail, setNewEmail] = useState("");
  const [newPassword, setNewPassword] = useState("");

  // Settings
  const [registrationEnabled, setRegistrationEnabled] = useState(false);
  const [maxApiKeys, setMaxApiKeys] = useState("10");

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

  // Details dialogs for mobile
  const [viewUser, setViewUser] = useState<User | null>(null);
  const [viewKey, setViewKey] = useState<APIKey | null>(null);

  // Database backup/restore
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [restoreConfirmDialog, setRestoreConfirmDialog] = useState<{
    isOpen: boolean;
    file: File | null;
  }>({ isOpen: false, file: null });
  const [backupCounts, setBackupCounts] = useState<{
    users: number;
    api_keys: number;
    documents: number;
  } | null>(null);

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
        setError(errorData.error || t("admin.errors.create_user"));
      }
    } catch (error) {
      setError(t("admin.errors.create_user"));
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
        setError(errorData.error || t("admin.errors.delete_user"));
      }
    } catch (error) {
      setError(t("admin.errors.delete_user"));
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
      setError(t("admin.errors.password_length"));
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
        setError(errorData.error || t("admin.errors.reset_password"));
      }
    } catch (error) {
      setError(t("admin.errors.reset_password"));
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
      setSmtpTestResult(null);

      const response = await fetch("/api/admin/test-smtp", {
        method: "POST",
        credentials: "include",
      });

      const result = await response.json();
      if (response.ok) {
        setError(null);
        setSmtpTestResult('working');
        // Revert back to default status after 3 seconds
        setTimeout(() => setSmtpTestResult(null), 3000);
      } else {
        setSmtpTestResult('failed');
        setTimeout(() => setSmtpTestResult(null), 3000);
        setError(result.error || t("admin.errors.smtp_test"));
      }
    } catch (error) {
      setSmtpTestResult('failed');
      setTimeout(() => setSmtpTestResult(null), 3000);
      setError(t("admin.errors.smtp_test_network"));
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
        setError(errorData.error || t("admin.errors.backup_create"));
      }
    } catch (error) {
      setError(t("admin.errors.backup_create"));
    } finally {
      setSaving(false);
    }
  };

  const analyzeBackupFile = async (file: File) => {
    try {
      setSaving(true);
      setError(null);
      setBackupCounts(null);

      const formData = new FormData();
      formData.append("backup_file", file);

      const response = await fetch("/api/admin/backup/analyze", {
        method: "POST",
        credentials: "include",
        body: formData,
      });

      const result = await response.json();
      if (response.ok && result.valid) {
        setBackupCounts({
          users: result.metadata.user_count,
          api_keys: result.metadata.api_key_count,
          documents: result.metadata.document_count,
        });
        return true;
      } else {
        setError(result.error || t("admin.errors.backup_invalid"));
        return false;
      }
    } catch (error) {
      setError(t("admin.errors.backup_analyze") + error.message);
      return false;
    } finally {
      setSaving(false);
    }
  };

  const handleRestoreFileSelect = async (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = event.target.files?.[0];
    if (file) {
      // Validate file type - only check filename extension
      const fileName = file.name.toLowerCase();
      const isTarGz = fileName.endsWith('.tar.gz') || fileName.endsWith('.tgz');
      
      if (!isTarGz) {
        setError(t("admin.errors.backup_file_type") + `"${file.name}"`);
        event.target.value = "";
        return;
      }
      
      // Analyze the backup file first
      const isValid = await analyzeBackupFile(file);
      if (isValid) {
        setRestoreConfirmDialog({ isOpen: true, file });
      }
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
      setSuccessMessage(null);

      const formData = new FormData();
      formData.append("backup_file", file);
      formData.append("overwrite_files", "true");
      formData.append("overwrite_database", "true");

      const response = await fetch("/api/admin/restore", {
        method: "POST",
        credentials: "include",
        body: formData,
      });

      const result = await response.json();
      if (response.ok) {
        setRestoreConfirmDialog({ isOpen: false, file: null });
        setError(null);
        let message = result.message || "Database restored successfully";
        if (result.metadata) {
          message += ` (${result.metadata.users_restored} users, exported ${result.metadata.export_date})`;
        }
        setSuccessMessage(message);
        // Refresh system status after restore
        await fetchSystemStatus();
        await fetchUsers();
        await fetchAPIKeys();
      } else {
        setError(result.error || t("admin.errors.restore_failed"));
      }
    } catch (error) {
      setError(t("admin.errors.restore_error") + error.message);
    } finally {
      setSaving(false);
    }
  };

  const closeRestoreConfirmDialog = () => {
    setRestoreConfirmDialog({ isOpen: false, file: null });
    setBackupCounts(null);
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
    // If we have a test result, show that instead
    if (smtpTestResult === 'working') {
      return "default";
    }
    if (smtpTestResult === 'failed') {
      return "destructive";
    }
    
    // Otherwise show configuration status
    switch (status) {
      case "configured":
        return "secondary";
      case "not_configured":
        return "secondary";
      default:
        return "secondary";
    }
  };

  const getSMTPStatusText = (status: string) => {
    // If we have a test result, show that instead
    if (smtpTestResult === 'working') {
      return t("admin.status.working");
    }
    if (smtpTestResult === 'failed') {
      return t("admin.status.failed");
    }
    
    // Otherwise show configuration status
    switch (status) {
      case "configured":
        return t("admin.status.configured");
      case "not_configured":
        return t("admin.status.not_configured");
      default:
        return t("admin.status.unknown");
    }
  };

  const getUserStatusBadge = (user: User) => {
    if (!user.is_active) {
      return (
        <Badge variant="secondary" className="w-16 justify-center">
          {t("admin.status.inactive")}
        </Badge>
      );
    }
    
    if (user.rmapi_paired) {
      return (
        <Badge variant="default" className="w-16 justify-center bg-primary text-primary-foreground hover:bg-primary/80">
          {t("admin.status.paired")}
        </Badge>
      );
    }
    
    return (
      <Badge variant="default" className="w-16 justify-center">
        {t("admin.status.unpaired")}
      </Badge>
    );
  };

  const isCurrentUser = (user: User) => {
    return currentUser && user.id === currentUser.id;
  };

  if (!systemStatus) {
    return (
      <Dialog open={isOpen} onOpenChange={onClose}>
        <DialogContent className="max-w-6xl">
          <div className="flex items-center justify-center p-8">{t("admin.loading_states.loading")}</div>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <Dialog open={isOpen} onOpenChange={handleClose}>
      <DialogContent className="max-w-7xl max-h-[85vh] overflow-y-auto sm:max-w-7xl sm:max-h-[90vh]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Shield className="h-5 w-5" />
            {t("admin.title")}
          </DialogTitle>
          <DialogDescription>
            {t("admin.description")}
          </DialogDescription>
        </DialogHeader>

        {error && (
          <Alert variant="destructive">
            <AlertTriangle className="h-4 w-4" />
            <AlertDescription>
              {error}
            </AlertDescription>
          </Alert>
        )}

        {successMessage && (
          <Alert>
            <CheckCircle className="h-4 w-4" />
            <AlertDescription>
              {successMessage}
            </AlertDescription>
          </Alert>
        )}

        <Tabs defaultValue="overview" className="w-full h-[600px] flex flex-col">
          <TabsList className="grid w-full grid-cols-5">
            <TabsTrigger value="overview" className="flex items-center gap-2">
              <Activity className="h-4 w-4" />
              <span className="hidden sm:inline">{t("admin.tabs.overview")}</span>
            </TabsTrigger>
            <TabsTrigger value="users" className="flex items-center gap-2">
              <Users className="h-4 w-4" />
              <span className="hidden sm:inline">{t("admin.tabs.users")}</span>
            </TabsTrigger>
            <TabsTrigger value="api-keys" className="flex items-center gap-2">
              <Key className="h-4 w-4" />
              <span className="hidden sm:inline">{t("admin.tabs.api_keys")}</span>
            </TabsTrigger>
            <TabsTrigger value="settings" className="flex items-center gap-2">
              <SettingsIcon className="h-4 w-4" />
              <span className="hidden sm:inline">{t("admin.tabs.settings")}</span>
            </TabsTrigger>
            <TabsTrigger value="system" className="flex items-center gap-2">
              <Database className="h-4 w-4" />
              <span className="hidden sm:inline">{t("admin.tabs.system")}</span>
            </TabsTrigger>
          </TabsList>

          <TabsContent value="overview" className="flex-1 overflow-y-auto">
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">
                    {t("admin.cards.total_users")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {systemStatus.database.total_users}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {systemStatus.database.active_users} {t("admin.status.active")}
                  </p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">
                    {t("admin.cards.api_keys")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {systemStatus.database.api_keys.total}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {systemStatus.database.api_keys.active} {t("admin.status.active")}
                  </p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">
                    {t("admin.cards.documents")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {systemStatus.database.documents}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {t("admin.descriptions.total_uploaded")}
                  </p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">
                    {t("admin.cards.active_sessions")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">
                    {systemStatus.database.active_sessions}
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {t("admin.descriptions.current_sessions")}
                  </p>
                </CardContent>
              </Card>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-4">
              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Mail className="h-5 w-5" />
                    {t("admin.cards.smtp_status")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center justify-between">
                    <Badge
                      variant={getSMTPStatusColor(systemStatus.smtp.status)}
                    >
                      {getSMTPStatusText(systemStatus.smtp.status)}
                    </Badge>
                    <Button size="sm" onClick={testSMTP} disabled={saving || !systemStatus.smtp.configured}>
                      {t("admin.actions.test_smtp")}
                    </Button>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle className="flex items-center gap-2">
                    <Server className="h-5 w-5" />
                    {t("admin.cards.system_mode")}
                  </CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="flex flex-col gap-2 items-start">
                    <Badge variant="secondary">{t("admin.badges.multi_user")}</Badge>
                    {systemStatus.dry_run && (
                      <Badge variant="destructive">{t("admin.badges.dry_run")}</Badge>
                    )}
                    {systemStatus.auth?.oidc_enabled && (
                      <Badge variant="secondary">{t("admin.badges.oidc_enabled")}</Badge>
                    )}
                    {systemStatus.auth?.proxy_auth_enabled && (
                      <Badge variant="secondary">{t("admin.badges.proxy_auth")}</Badge>
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
                  <CardTitle>{t("admin.cards.create_new_user")}</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-3 gap-4">
                    <div>
                      <Label htmlFor="new-username">{t("admin.labels.username")}</Label>
                      <Input
                        id="new-username"
                        value={newUsername}
                        onChange={(e) => setNewUsername(e.target.value)}
                        placeholder={t("admin.placeholders.username")}
                        className="mt-2"
                      />
                    </div>
                    <div>
                      <Label htmlFor="new-email">{t("admin.labels.email")}</Label>
                      <Input
                        id="new-email"
                        type="email"
                        value={newEmail}
                        onChange={(e) => setNewEmail(e.target.value)}
                        placeholder={t("admin.placeholders.email")}
                        className="mt-2"
                      />
                    </div>
                    <div>
                      <Label htmlFor="new-password">{t("admin.labels.password")}</Label>
                      <Input
                        id="new-password"
                        type="password"
                        value={newPassword}
                        onChange={(e) => setNewPassword(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter' && newUsername && newEmail && newPassword && !saving) {
                            createUser();
                          }
                        }}
                        placeholder={t("admin.placeholders.password")}
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
                      {t("admin.actions.create_user")}
                    </Button>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>{t("admin.counts.users", {count: users.length})}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("admin.labels.username")}</TableHead>
                          <TableHead className="hidden md:table-cell">{t("admin.labels.email")}</TableHead>
                          <TableHead className="hidden md:table-cell">{t("admin.labels.role")}</TableHead>
                          <TableHead className="hidden md:table-cell">{t("admin.labels.status")}</TableHead>
                          <TableHead className="hidden md:table-cell">{t("admin.labels.created")}</TableHead>
                          <TableHead className="hidden md:table-cell">{t("admin.labels.last_login")}</TableHead>
                          <TableHead>{t("admin.labels.actions")}</TableHead>
                        </TableRow>
                      </TableHeader>
                    <TableBody>
                      {users.map((user) => (
                        <TableRow key={user.id}>
                          <TableCell className="font-medium max-w-24 md:max-w-none">
                            <div className="truncate" title={user.username}>
                              {user.username}
                            </div>
                          </TableCell>
                          <TableCell className="hidden md:table-cell">{user.email}</TableCell>
                          <TableCell className="hidden md:table-cell">
                            <Badge
                              variant={user.is_admin ? "default" : "secondary"}
                              className="w-14 justify-center"
                            >
                              {user.is_admin ? t("admin.roles.admin") : t("admin.roles.user")}
                            </Badge>
                          </TableCell>
                          <TableCell className="hidden md:table-cell">
                            {getUserStatusBadge(user)}
                          </TableCell>
                          <TableCell className="hidden md:table-cell">{formatDate(user.created_at)}</TableCell>
                          <TableCell className="hidden md:table-cell">
                            {user.last_login
                              ? formatDate(user.last_login)
                              : t("admin.never")}
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2 flex-wrap">
                              <Button
                                size="sm"
                                variant="outline"
                                className="md:hidden"
                                onClick={() => setViewUser(user)}
                              >
                                {t('admin.actions.details', 'Details')}
                              </Button>
                              <Button
                                size="sm"
                                variant="outline"
                                onClick={() => openResetPasswordDialog(user)}
                                className="whitespace-nowrap"
                              >
                                {t("admin.actions.reset_password")}
                              </Button>
                              {!isCurrentUser(user) && (
                                <>
                                  <Button
                                    size="sm"
                                    variant="outline"
                                    onClick={() =>
                                      toggleAdminStatus(user.id, !user.is_admin)
                                    }
                                    className="whitespace-nowrap"
                                  >
                                    {user.is_admin ? t("admin.actions.make_user") : t("admin.actions.make_admin")}
                                  </Button>
                                  <Button
                                    size="sm"
                                    variant="outline"
                                    onClick={() =>
                                      toggleUserStatus(user.id, !user.is_active)
                                    }
                                    className="whitespace-nowrap"
                                  >
                                    {user.is_active ? t("admin.actions.deactivate") : t("admin.actions.activate")}
                                  </Button>
                                  <Button
                                    size="sm"
                                    variant="destructive"
                                    onClick={() => openDeleteUserDialog(user)}
                                    className="whitespace-nowrap"
                                  >
                                    {t("admin.actions.delete")}
                                  </Button>
                                </>
                              )}
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                  </div>
                </CardContent>
                  </Card>
                  </div>
                  </TabsContent>

          <TabsContent value="api-keys" className="flex-1 overflow-y-auto">
            <div className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>{t("admin.counts.all_api_keys", {count: apiKeys.length})}</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("admin.labels.name")}</TableHead>
                          <TableHead>{t("admin.labels.user")}</TableHead>
                          <TableHead className="hidden md:table-cell">{t("admin.labels.key_preview")}</TableHead>
                          <TableHead className="hidden md:table-cell">{t("admin.labels.status")}</TableHead>
                          <TableHead className="hidden md:table-cell">{t("admin.labels.created")}</TableHead>
                          <TableHead className="hidden md:table-cell">{t("admin.labels.last_used")}</TableHead>
                          <TableHead className="hidden md:table-cell">{t("admin.labels.expires")}</TableHead>
                          <TableHead className="md:hidden">{t("admin.labels.actions")}</TableHead>
                        </TableRow>
                      </TableHeader>
                  <TableBody>
                    {apiKeys.map((key) => {
                      const status = getKeyStatus(key);
                      return (
                        <TableRow key={key.id}>
                          <TableCell className="font-medium max-w-20 md:max-w-none">
                            <div className="truncate" title={key.name}>
                              {key.name}
                            </div>
                          </TableCell>
                          <TableCell className="max-w-20 md:max-w-none">
                            <div className="truncate" title={key.username}>
                              {key.username}
                            </div>
                          </TableCell>
                          <TableCell className="hidden md:table-cell">
                            <code className="text-sm">{key.key_prefix}...</code>
                          </TableCell>
                          <TableCell className="hidden md:table-cell">
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
                          <TableCell className="hidden md:table-cell">{formatDate(key.created_at)}</TableCell>
                          <TableCell className="hidden md:table-cell">
                            {key.last_used
                              ? formatDate(key.last_used)
                              : t("admin.never")}
                          </TableCell>
                          <TableCell className="hidden md:table-cell">
                            {key.expires_at
                              ? formatDate(key.expires_at)
                              : t("admin.never")}
                          </TableCell>
                          <TableCell className="md:hidden">
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() => setViewKey(key)}
                            >
                              {t('admin.actions.details', 'Details')}
                            </Button>
                          </TableCell>
                        </TableRow>
                      );
                    })}
                  </TableBody>
                </Table>
                  </div>
                </CardContent>
              </Card>
            </div>
          </TabsContent>

          <TabsContent value="settings" className="flex-1 overflow-y-auto">
            <div className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>{t("admin.cards.user_management")}</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="flex items-center justify-between">
                    <div className="space-y-1">
                      <Label htmlFor="registration-enabled">
                        {t("admin.labels.enable_registration")}
                      </Label>
                      <p className="text-sm text-muted-foreground">
                        {t("admin.descriptions.registration_help")}
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
                  <CardTitle>{t("admin.cards.api_key_settings")}</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div>
                    <Label htmlFor="max-api-keys">
                      {t("admin.labels.max_api_keys")}
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

            </div>
          </TabsContent>

          <TabsContent value="system" className="flex-1 overflow-y-auto">
            <div className="space-y-4">
              <Card>
                <CardHeader>
                  <CardTitle>{t("admin.cards.backup_restore")}</CardTitle>
                </CardHeader>
                <CardContent className="space-y-4">
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    <Button
                      variant="outline"
                      size="lg"
                      onClick={handleBackupDatabase}
                      disabled={saving}
                      className="w-full"
                    >
                      <Database className="h-4 w-4 mr-2" />
                      {saving ? t("admin.loading_states.creating_backup") : t("admin.actions.create_backup")}
                    </Button>
                    <div className="w-full">
                      <input
                        type="file"
                        ref={fileInputRef}
                        onChange={handleRestoreFileSelect}
                        accept=".tar.gz,.tgz,application/gzip,application/x-gzip,application/x-tar,application/x-compressed-tar"
                        style={{ display: "none" }}
                      />
                      <Button
                        variant="outline"
                        size="lg"
                        onClick={() => fileInputRef.current?.click()}
                        disabled={saving}
                        className="w-full"
                      >
                        <Database className="h-4 w-4 mr-2" />
                        {saving ? t("admin.loading_states.analyzing") : t("admin.actions.restore_backup")}…
                      </Button>
                    </div>
                  </div>
                  <div className="text-sm text-muted-foreground space-y-2">
                    <p>
                      <strong>{t("admin.actions.create_backup")}:</strong> {t("admin.descriptions.backup_description")}
                    </p>
                    <p>
                      <strong>{t("admin.actions.restore_backup")}:</strong> {t("admin.descriptions.restore_description")}
                    </p>
                    <div className="flex items-center gap-1 text-muted-foreground">
                      <AlertTriangle className="h-4 w-4" />
                      <span>{t("admin.descriptions.restore_warning")}</span>
                    </div>
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
              {t("admin.dialogs.reset_password_title")}
            </DialogTitle>
            <DialogDescription>
              {t("admin.dialogs.reset_password_description")}
              <strong>{resetPasswordDialog.user?.username}</strong>
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div>
              <Label htmlFor="new-password">{t("admin.labels.new_password")}</Label>
              <Input
                id="new-password"
                type="password"
                value={newPasswordValue}
                onChange={(e) => setNewPasswordValue(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && newPasswordValue.length >= 8 && !saving) {
                    confirmResetPassword();
                  }
                }}
                placeholder={t("admin.placeholders.new_password")}
                className="mt-2"
              />
            </div>
          </div>

          <DialogFooter>
            <Button variant="outline" onClick={closeResetPasswordDialog}>
              {t("admin.actions.cancel")}
            </Button>
            <Button
              onClick={confirmResetPassword}
              disabled={saving || newPasswordValue.length < 8}
            >
              {saving ? t("admin.loading_states.resetting") : t("admin.actions.reset_password")}
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
              <AlertTriangle className="h-5 w-5 text-muted-foreground" />
              {t("admin.dialogs.confirm_restore_title")}
            </AlertDialogTitle>
            <AlertDialogDescription className="space-y-3">
              <p>
                {t("admin.dialogs.confirm_restore_description")}
                <strong>{restoreConfirmDialog.file?.name}</strong>
              </p>
              
              {backupCounts && (
                <div className="bg-muted p-3 rounded-md">
                  <p className="font-medium text-sm mb-2">{t("admin.dialogs.backup_contents")}</p>
                  <div className="grid grid-cols-3 gap-4 text-sm">
                    <div className="text-center">
                      <div className="font-semibold text-lg">{backupCounts.users}</div>
                      <div className="text-muted-foreground">{t("admin.labels.users")}</div>
                    </div>
                    <div className="text-center">
                      <div className="font-semibold text-lg">{backupCounts.api_keys}</div>
                      <div className="text-muted-foreground">{t("admin.labels.api_keys")}</div>
                    </div>
                    <div className="text-center">
                      <div className="font-semibold text-lg">{backupCounts.documents}</div>
                      <div className="text-muted-foreground">{t("admin.labels.documents")}</div>
                    </div>
                  </div>
                </div>
              )}
              
              <p className="text-destructive font-medium">
                {t("admin.dialogs.restore_warning_text")}
              </p>
              <ul className="list-disc list-inside text-sm space-y-1 ml-4">
                <li>{t("admin.dialogs.restore_warning_items.accounts")}</li>
                <li>{t("admin.dialogs.restore_warning_items.api_keys")}</li>
                <li>{t("admin.dialogs.restore_warning_items.documents")}</li>
                <li>{t("admin.dialogs.restore_warning_items.pairing")}</li>
                <li>{t("admin.dialogs.restore_warning_items.folders")}</li>
                <li>{t("admin.dialogs.restore_warning_items.settings")}</li>
              </ul>
              <p className="text-destructive font-medium">
                {t("admin.dialogs.restore_final_warning")}
              </p>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={saving}>{t("admin.actions.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDatabaseRestore}
              disabled={saving}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {saving ? t("admin.loading_states.restoring") : t("admin.actions.restore_database")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* User Details Dialog */}
      <Dialog open={!!viewUser} onOpenChange={() => setViewUser(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{viewUser?.username}</DialogTitle>
          </DialogHeader>
          {viewUser && (
            <div className="space-y-2 text-sm">
              <p>
                <strong>{t('admin.labels.email')}:</strong> {viewUser.email}
              </p>
              <p>
                <strong>{t('admin.labels.role')}:</strong>{' '}
                {viewUser.is_admin ? t('admin.roles.admin') : t('admin.roles.user')}
              </p>
              <p>
                <strong>{t('admin.labels.status')}:</strong>{' '}
                {viewUser.is_active ? t('admin.status.active') : t('admin.status.inactive')}
              </p>
              <p>
                <strong>{t('admin.labels.created')}:</strong> {formatDate(viewUser.created_at)}
              </p>
              <p>
                <strong>{t('admin.labels.last_login')}:</strong>{' '}
                {viewUser.last_login ? formatDate(viewUser.last_login) : t('admin.never')}
              </p>
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* API Key Details Dialog */}
      <Dialog open={!!viewKey} onOpenChange={() => setViewKey(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{viewKey?.name}</DialogTitle>
          </DialogHeader>
          {viewKey && (
            <div className="space-y-2 text-sm">
              <p>
                <strong>{t('admin.labels.user')}:</strong> {viewKey.username}
              </p>
              <p>
                <strong>{t('admin.labels.key_preview')}:</strong>{' '}
                <code>{viewKey.key_prefix}...</code>
              </p>
              <p>
                <strong>{t('admin.labels.status')}:</strong> {getKeyStatus(viewKey)}
              </p>
              <p>
                <strong>{t('admin.labels.created')}:</strong> {formatDate(viewKey.created_at)}
              </p>
              <p>
                <strong>{t('admin.labels.last_used')}:</strong>{' '}
                {viewKey.last_used ? formatDate(viewKey.last_used) : t('admin.never')}
              </p>
              <p>
                <strong>{t('admin.labels.expires')}:</strong>{' '}
                {viewKey.expires_at ? formatDate(viewKey.expires_at) : t('admin.never')}
              </p>
            </div>
          )}
        </DialogContent>
      </Dialog>


    </Dialog>
  );
}
