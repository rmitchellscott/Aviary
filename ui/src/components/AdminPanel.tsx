import React, { useState, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";
import { useUserData } from "@/hooks/useUserData";
import { useConfig } from "@/hooks/useConfig";
import { useAuth } from "@/components/AuthProvider";
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
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import {
  Tooltip,
  TooltipTrigger,
  TooltipContent,
} from "@/components/ui/tooltip";
import { Progress } from "@/components/ui/progress";
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
  Loader2,
  Download,
  Trash2,
} from "lucide-react";

/**
 * Compare two semver version strings
 * Returns: 1 if v1 > v2, -1 if v1 < v2, 0 if equal
 * Returns null if either version is invalid
 */
function compareSemver(v1: string | null | undefined, v2: string | null | undefined): number | null {
  if (!v1 || !v2) return null;
  
  // Remove 'v' prefix if present
  const clean1 = v1.replace(/^v/, '');
  const clean2 = v2.replace(/^v/, '');
  
  const parts1 = clean1.split('.').map(p => parseInt(p, 10));
  const parts2 = clean2.split('.').map(p => parseInt(p, 10));
  
  if (parts1.length < 3 || parts2.length < 3 || 
      parts1.some(isNaN) || parts2.some(isNaN)) {
    return null;
  }
  
  for (let i = 0; i < 3; i++) {
    if (parts1[i] > parts2[i]) return 1;
    if (parts1[i] < parts2[i]) return -1;
  }
  
  return 0;
}

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

interface BackupJob {
  id: string;
  status: string;
  progress: number;
  include_files: boolean;
  include_configs: boolean;
  file_path?: string;
  filename?: string;
  file_size?: number;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  expires_at?: string;
  created_at: string;
}

interface RestoreUpload {
  id: string;
  filename: string;
  file_size: number;
  status: string;
  expires_at: string;
  created_at: string;
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
  const { config } = useConfig();
  const { logout } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [systemStatus, setSystemStatus] = useState<SystemStatus | null>(null);
  const [backupJobs, setBackupJobs] = useState<BackupJob[]>([]);
  const [restoreUploads, setRestoreUploads] = useState<RestoreUpload[]>([]);
  const [loading, setLoading] = useState(false);
  const [creatingBackup, setCreatingBackup] = useState(false);
  const [restoringBackup, setRestoringBackup] = useState(false);
  const [testingSMTP, setTestingSMTP] = useState(false);
  const [creatingUser, setCreatingUser] = useState(false);
  const [resettingPassword, setResettingPassword] = useState(false);
  const [analyzingBackup, setAnalyzingBackup] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  // SMTP test state
  const [smtpTestResult, setSmtpTestResult] = useState<'working' | 'failed' | null>(null);
  // User creation form
  const [newUsername, setNewUsername] = useState("");
  const [newEmail, setNewEmail] = useState("");
  const [newPassword, setNewPassword] = useState("");

  const [registrationEnabled, setRegistrationEnabled] = useState(false);
  const [maxApiKeys, setMaxApiKeys] = useState("10");
  const [maxApiKeysError, setMaxApiKeysError] = useState<string | null>(null);

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

  const [viewUser, setViewUser] = useState<User | null>(null);
  const [viewKey, setViewKey] = useState<APIKey | null>(null);

  const [deleteBackupDialog, setDeleteBackupDialog] = useState<{
    isOpen: boolean;
    job: BackupJob | null;
  }>({ isOpen: false, job: null });

  // Database backup/restore
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [restoreConfirmDialog, setRestoreConfirmDialog] = useState<{
    isOpen: boolean;
    upload: RestoreUpload | null;
  }>({ isOpen: false, upload: null });
  const [backupCounts, setBackupCounts] = useState<{
    users: number;
    api_keys: number;
    documents: number;
  } | null>(null);
  const [backupVersion, setBackupVersion] = useState<string | null>(null);
  const [versionInfo, setVersionInfo] = useState<{
    version: string;
    git_commit: string;
    build_date: string;
    go_version: string;
  } | null>(null);
  const [uploadProgress, setUploadProgress] = useState<number>(0);
  const [uploadPhase, setUploadPhase] = useState<'idle' | 'uploading' | 'extracting' | 'validating'>('idle');
  const [downloadingJobId, setDownloadingJobId] = useState<string | null>(null);
  const [restorePerformed, setRestorePerformed] = useState(false);

  useEffect(() => {
    if (isOpen) {
      setRestorePerformed(false);
      fetchSystemStatus();
      fetchUsers();
      fetchAPIKeys();
      fetchBackupJobs();
      fetchRestoreUploads();
      fetchVersionInfo();
    }
  }, [isOpen]);

  useEffect(() => {
    let interval: NodeJS.Timeout;
    const hasActiveJobs = 
      backupJobs.some(job => job.status === 'running' || job.status === 'pending');
      
    if (isOpen && hasActiveJobs) {
      interval = setInterval(() => {
        fetchBackupJobs();
      }, 2000);
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [isOpen, backupJobs]);

  // Listen for logout event to clear sensitive admin state
  useEffect(() => {
    const handleLogout = () => {
      // Clear all sensitive admin state
      setUsers([]);
      setApiKeys([]);
      setSystemStatus(null);
      setBackupJobs([]);
      setRestoreUploads([]);
      setError(null);
      setSuccessMessage(null);
      setSmtpTestResult(null);
      
      // Clear form state
      setNewUsername("");
      setNewEmail("");
      setNewPassword("");
      setRegistrationEnabled(false);
      setMaxApiKeys("10");
      setMaxApiKeysError(null);
      setNewPasswordValue("");
      setDeleting(false);
      setBackupCounts({ users: 0, api_keys: 0, documents: 0 });
      setBackupVersion(null);
      setRestorePerformed(false);
      setVersionInfo({ version: "", gitCommit: "", buildDate: "", goVersion: "" });
      setUploadProgress(0);
      setUploadPhase('idle');
      setDownloadingJobId(null);
      
      // Close any open dialogs
      setResetPasswordDialog({ isOpen: false, user: null });
      setDeleteUserDialog({ isOpen: false, user: null });
      setDeleteBackupDialog({ isOpen: false, job: null });
      setRestoreConfirmDialog({ isOpen: false, upload: null });
      setViewUser(null);
      setViewKey(null);
      
      // Reset loading states
      setLoading(false);
      setCreatingBackup(false);
      setRestoringBackup(false);
      setTestingSMTP(false);
      setCreatingUser(false);
      setResettingPassword(false);
      setAnalyzingBackup(false);
    };

    window.addEventListener('logout', handleLogout);

    return () => {
      window.removeEventListener('logout', handleLogout);
    };
  }, []);

  const fetchWithSessionCheck = async (url: string, options?: RequestInit) => {
    const response = await fetch(url, options);
    if (response.status === 401) {
      logout();
      window.location.href = '/';
      throw new Error('Session expired after restore. Please log in again.');
    }
    return response;
  };

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

  const fetchBackupJobs = async () => {
    try {
      const response = await fetch("/api/admin/backup-jobs", {
        credentials: "include",
      });

      if (response.ok) {
        const data = await response.json();
        setBackupJobs(data.jobs || []);
      }
    } catch (error) {
      console.error("Failed to fetch backup jobs:", error);
    }
  };

  const fetchRestoreUploads = async () => {
    try {
      const response = await fetch("/api/admin/restore/uploads", {
        credentials: "include",
      });

      if (response.ok) {
        const data = await response.json();
        setRestoreUploads(data.uploads || []);
      }
    } catch (error) {
      console.error("Failed to fetch restore uploads:", error);
    }
  };

  const fetchVersionInfo = async () => {
    try {
      const response = await fetch("/api/version", {
        credentials: "include",
      });

      if (response.ok) {
        const data = await response.json();
        setVersionInfo(data);
      }
    } catch (error) {
      console.error("Failed to fetch version info:", error);
    }
  };

  const createUser = async () => {
    try {
      setCreatingUser(true);
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
      setCreatingUser(false);
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
    
    if (restorePerformed) {
      window.location.reload();
    } else {
      onClose();
    }
  };

  const confirmResetPassword = async () => {
    if (!resetPasswordDialog.user || !newPasswordValue) return;

    if (newPasswordValue.length < 8) {
      setError(t("admin.errors.password_length"));
      return;
    }

    try {
      setResettingPassword(true);
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
      setResettingPassword(false);
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
      setTestingSMTP(true);
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
      setTestingSMTP(false);
    }
  };

  const handleCreateBackupJob = async () => {
    try {
      setCreatingBackup(true);
      setError(null);

      const response = await fetch("/api/admin/backup-job", {
        method: "POST",
        credentials: "include",
      });

      if (response.ok) {
        const data = await response.json();
        setSuccessMessage(t("admin.messages.backup_job_created"));
        await fetchBackupJobs();
      } else {
        const errorData = await response.json();
        setError(errorData.error_type ? t(`admin.errors.${errorData.error_type}`) : t("admin.errors.backup_create"));
      }
    } catch (error) {
      setError(t("admin.errors.backup_create"));
    } finally {
      setCreatingBackup(false);
    }
  };


  const handleDownloadBackup = async (jobId: string) => {
    try {
      setDownloadingJobId(jobId);
      const response = await fetch(`/api/admin/backup-job/${jobId}/download`, {
        credentials: "include",
      });

      if (response.ok) {
        const contentDisposition = response.headers.get("Content-Disposition");
        let filename = "backup.tar.gz";
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
        setError(errorData.error_type ? t(`admin.errors.${errorData.error_type}`) : t("admin.errors.backup_download"));
      }
    } catch (error) {
      setError(t("admin.errors.backup_download"));
    } finally {
      setDownloadingJobId(null);
    }
  };

  const openDeleteBackupDialog = (job: BackupJob) => {
    setDeleteBackupDialog({ isOpen: true, job });
  };

  const closeDeleteBackupDialog = () => {
    setDeleteBackupDialog({ isOpen: false, job: null });
  };

  const confirmDeleteBackup = async () => {
    if (!deleteBackupDialog.job) return;

    try {
      setDeleting(true);
      const response = await fetch(`/api/admin/backup-job/${deleteBackupDialog.job.id}`, {
        method: "DELETE",
        credentials: "include",
      });

      if (response.ok) {
        await fetchBackupJobs();
        closeDeleteBackupDialog();
      } else {
        const errorData = await response.json();
        setError(errorData.error_type ? t(`admin.errors.${errorData.error_type}`) : t("admin.backup.delete_error"));
      }
    } catch (error) {
      setError(t("admin.backup.delete_error"));
    } finally {
      setDeleting(false);
    }
  };


  const uploadRestoreFileWithProgress = async (formData: FormData): Promise<Response> => {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      
      xhr.upload.addEventListener('progress', (event) => {
        if (event.lengthComputable) {
          const percentComplete = (event.loaded / event.total) * 100;
          setUploadProgress(percentComplete);
        }
      });
      
      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          const response = new Response(xhr.responseText, {
            status: xhr.status,
            statusText: xhr.statusText,
            headers: {
              'Content-Type': 'application/json',
            },
          });
          resolve(response);
        } else {
          reject(new Error(xhr.responseText || `HTTP ${xhr.status}`));
        }
      });
      
      xhr.addEventListener('error', () => {
        reject(new Error('Upload failed'));
      });
      
      xhr.open('POST', '/api/admin/restore/upload');
      xhr.withCredentials = true;
      
      xhr.send(formData);
    });
  };

  const analyzeBackupFile = async (file: File) => {
    try {
      setAnalyzingBackup(true);
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
        setError(result.error_type ? t(`admin.errors.${result.error_type}`) : t("admin.errors.backup_invalid"));
        return false;
      }
    } catch (error) {
      setError(t("admin.errors.backup_analyze") + error.message);
      return false;
    } finally {
      setAnalyzingBackup(false);
    }
  };

  const handleRestoreFileSelect = async (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = event.target.files?.[0];
    if (file) {
      const fileName = file.name.toLowerCase();
      const isTarGz = fileName.endsWith('.tar.gz') || fileName.endsWith('.tgz');
      
      if (!isTarGz) {
        setError(t("admin.errors.backup_file_type") + `"${file.name}"`);
        event.target.value = "";
        return;
      }
      
      try {
        setError(null);
        setUploadProgress(0);
        setUploadPhase('idle');

        const formData = new FormData();
        formData.append("backup_file", file);

        setUploadPhase('uploading');
        const response = await uploadRestoreFileWithProgress(formData);
        
        if (response.ok) {
          const responseData = await response.json();
          const uploadId = responseData.upload_id;
          
          setUploadPhase('extracting');
          setUploadProgress(100);
          
          if (uploadId) {
            try {
              setUploadPhase('validating');
              
              const analyzeResponse = await fetch(`/api/admin/restore/uploads/${uploadId}/analyze`, {
                method: "POST",
                credentials: "include",
              });
              
              const analyzeResult = await analyzeResponse.json();
              if (analyzeResponse.ok && analyzeResult.valid) {
                setBackupCounts({
                  users: analyzeResult.metadata.user_count,
                  api_keys: analyzeResult.metadata.api_key_count,
                  documents: analyzeResult.metadata.document_count,
                });
                setBackupVersion(analyzeResult.metadata.aviary_version || null);
              }
            } catch (error) {
              console.error("Failed to analyze backup during upload:", error);
            }
          }
          
          await fetchRestoreUploads();
          
          setUploadProgress(0);
          setUploadPhase('idle');
        } else {
          const errorData = await response.json();
          setError(errorData.error_type ? t(`admin.errors.${errorData.error_type}`) : t("admin.errors.restore_failed"));
          setUploadProgress(0);
          setUploadPhase('idle');
        }
      } catch (error) {
        setError(t("admin.errors.restore_failed"));
        setUploadProgress(0);
        setUploadPhase('idle');
      }
    }
    event.target.value = "";
  };

  const confirmDatabaseRestore = async () => {
    const upload = restoreConfirmDialog.upload;
    if (!upload) return;

    try {
      setRestoringBackup(true);
      setError(null);
      setSuccessMessage(null);

      const response = await fetch("/api/admin/restore", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        credentials: "include",
        body: JSON.stringify({
          upload_id: upload.id,
          overwrite_files: true,
          overwrite_database: true,
          user_ids: []
        }),
      });

      const result = await response.json();
      if (response.ok) {
        setRestoreConfirmDialog({ isOpen: false, upload: null });
        setError(null);
        const filename = restoreConfirmDialog.upload?.filename || 'backup file';
        const message = t("admin.success.backup_restored", { filename });
        setSuccessMessage(message);
        setRestorePerformed(true);
        try {
          await fetchWithSessionCheck("/api/admin/status", { credentials: "include" });
          await fetchWithSessionCheck("/api/users", { credentials: "include" });
          await fetchWithSessionCheck("/api/admin/api-keys", { credentials: "include" });
          await fetchWithSessionCheck("/api/admin/backup-jobs", { credentials: "include" });
          await fetchWithSessionCheck("/api/admin/restore/uploads", { credentials: "include" });
          
          await fetchSystemStatus();
          await fetchUsers();
          await fetchAPIKeys();
          await fetchBackupJobs();
          await fetchRestoreUploads();
        } catch (error) {
          return;
        }
      } else {
        setError(result.error_type ? t(`admin.errors.${result.error_type}`) : t("admin.errors.restore_failed"));
      }
    } catch (error) {
      setError(t("admin.errors.restore_error") + error.message);
    } finally {
      setRestoringBackup(false);
    }
  };

  const handleRestoreUpload = async (upload: RestoreUpload) => {
    setRestoreConfirmDialog({ isOpen: true, upload });
  };

  const closeRestoreConfirmDialog = () => {
    setRestoreConfirmDialog({ isOpen: false, upload: null });
    setBackupCounts(null);
    setBackupVersion(null);
  };

  const cancelRestoreUpload = async () => {
    const upload = restoreConfirmDialog.upload;
    if (!upload) return;

    // Immediately clear restore uploads for instant UI feedback
    setRestoreUploads([]);
    closeRestoreConfirmDialog();

    try {
      const response = await fetch(`/api/admin/restore/uploads/${upload.id}`, {
        method: "DELETE",
        credentials: "include",
      });

      if (!response.ok) {
        const errorData = await response.json();
        setError(errorData.error_type ? t(`admin.errors.${errorData.error_type}`) : t("admin.errors.restore_cancel_failed"));
      }
      // Always refresh to get accurate state
      await fetchRestoreUploads();
    } catch (error) {
      setError(t("admin.errors.restore_cancel_failed"));
      // Re-fetch to restore state in case of error
      await fetchRestoreUploads();
    }
  };



  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleString();
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


  const getBackupStatusButton = (job: BackupJob) => {
    if (job.status === "completed" || job.status === "failed") {
      return (
        <div className="flex gap-2 shrink-0">
          <Button
            size="sm"
            variant="outline"
            onClick={() => handleDownloadBackup(job.id)}
            disabled={job.status === "failed" || downloadingJobId === job.id}
          >
            {downloadingJobId === job.id ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <>
                <Download className="h-4 w-4 sm:hidden" />
                <span className="hidden sm:inline">{t("admin.backup.download")}</span>
              </>
            )}
          </Button>
          <Button
            size="sm"
            variant="destructive"
            onClick={() => openDeleteBackupDialog(job)}
          >
            <Trash2 className="h-4 w-4 sm:hidden" />
            <span className="hidden sm:inline">{t("admin.actions.delete")}</span>
          </Button>
        </div>
      );
    }

    let content;

    switch (job.status) {
      case "pending":
      case "running":
        content = <Loader2 className="h-4 w-4 animate-spin" />;
        break;
      default:
        content = job.status;
    }

    return (
      <Button
        size="sm"
        variant="secondary"
        disabled
        className="shrink-0"
      >
        {content}
      </Button>
    );
  };

  const formatFileSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const getUserStatusBadge = (user: User) => {
    if (!user.is_active) {
      return (
        <Badge variant="secondary" className="min-w-16 max-w-32 justify-center text-center whitespace-nowrap">
          {t("admin.status.inactive")}
        </Badge>
      );
    }
    
    if (user.rmapi_paired) {
      return (
        <Badge variant="default" className="min-w-16 max-w-32 justify-center text-center whitespace-nowrap bg-primary text-primary-foreground hover:bg-primary/80">
          {t("admin.status.paired")}
        </Badge>
      );
    }
    
    return (
      <Badge variant="default" className="min-w-16 max-w-32 justify-center text-center whitespace-nowrap">
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
            <div className="grid grid-cols-2 md:grid-cols-2 lg:grid-cols-4 gap-4">
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
                    <Button size="sm" onClick={testSMTP} disabled={testingSMTP || !systemStatus.smtp.configured}>
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
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
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
                          if (e.key === 'Enter' && newUsername && newEmail && newPassword && !creatingUser) {
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
                        creatingUser || !newUsername || !newEmail || !newPassword
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
                    <Table className="w-full table-fixed lg:table-auto">
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("admin.labels.username")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("admin.labels.email")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("admin.labels.role")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("admin.labels.status")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("admin.labels.created")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("admin.labels.last_login")}</TableHead>
                          <TableHead>{t("admin.labels.actions")}</TableHead>
                        </TableRow>
                      </TableHeader>
                    <TableBody>
                      {users.map((user) => (
                        <TableRow key={user.id}>
                          <TableCell className="font-medium">
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <div className="truncate cursor-default" title={user.username}>
                                  {user.username}
                                </div>
                              </TooltipTrigger>
                              <TooltipContent>
                                {t('admin.labels.user_id')}: {user.id}
                              </TooltipContent>
                            </Tooltip>
                          </TableCell>
                          <TableCell className="hidden lg:table-cell">{user.email}</TableCell>
                          <TableCell className="hidden lg:table-cell text-center">
                            {config?.oidcGroupBasedAdmin ? (
                              <Tooltip>
                                <TooltipTrigger asChild>
                                  <Badge
                                    variant={user.is_admin ? "default" : "secondary"}
                                    className="min-w-14 max-w-24 justify-center cursor-default text-center whitespace-nowrap"
                                  >
                                    {user.is_admin ? t("admin.roles.admin") : t("admin.roles.user")}
                                  </Badge>
                                </TooltipTrigger>
                                <TooltipContent>
                                  {t("admin.tooltips.oidc_managed")}
                                </TooltipContent>
                              </Tooltip>
                            ) : (
                              <Badge
                                variant={user.is_admin ? "default" : "secondary"}
                                className="min-w-14 max-w-24 justify-center text-center whitespace-nowrap"
                              >
                                {user.is_admin ? t("admin.roles.admin") : t("admin.roles.user")}
                              </Badge>
                            )}
                          </TableCell>
                          <TableCell className="hidden lg:table-cell text-center">
                            {getUserStatusBadge(user)}
                          </TableCell>
                          <TableCell className="hidden lg:table-cell">{formatDate(user.created_at)}</TableCell>
                          <TableCell className="hidden lg:table-cell">
                            {user.last_login
                              ? formatDate(user.last_login)
                              : t("admin.never")}
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2 flex-wrap">
                              <Button
                                size="sm"
                                variant="outline"
                                className="lg:hidden"
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
                                  {!config?.oidcGroupBasedAdmin && (
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
                                  )}
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
                    <Table className="w-full table-fixed lg:table-auto">
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("admin.labels.name")}</TableHead>
                          <TableHead>{t("admin.labels.user")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("admin.labels.key_preview")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("admin.labels.status")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("admin.labels.created")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("admin.labels.last_used")}</TableHead>
                          <TableHead className="hidden lg:table-cell">{t("admin.labels.expires")}</TableHead>
                          <TableHead className="lg:hidden">{t("admin.labels.actions")}</TableHead>
                        </TableRow>
                      </TableHeader>
                  <TableBody>
                    {apiKeys.map((key) => {
                      const status = getKeyStatus(key);
                      return (
                        <TableRow key={key.id}>
                          <TableCell className="font-medium">
                            <div className="truncate" title={key.name}>
                              {key.name}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="truncate" title={key.username}>
                              {key.username}
                            </div>
                          </TableCell>
                          <TableCell className="hidden lg:table-cell">
                            <code className="text-sm">{key.key_prefix}...</code>
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
                          <TableCell className="hidden lg:table-cell">{formatDate(key.created_at)}</TableCell>
                          <TableCell className="hidden lg:table-cell">
                            {key.last_used
                              ? formatDate(key.last_used)
                              : t("admin.never")}
                          </TableCell>
                          <TableCell className="hidden lg:table-cell">
                            {key.expires_at
                              ? formatDate(key.expires_at)
                              : t("admin.never")}
                          </TableCell>
                          <TableCell className="lg:hidden">
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
                      min="1"
                      max="100"
                      value={maxApiKeys}
                      onChange={(e) => {
                        const value = e.target.value;
                        setMaxApiKeys(value);
                        
                        if (value === "") {
                          setMaxApiKeysError(null);
                          return;
                        }
                        
                        const numValue = parseInt(value, 10);
                        if (isNaN(numValue) || numValue < 1 || numValue > 100) {
                          setMaxApiKeysError(t("admin.errors.max_api_keys_invalid"));
                        } else {
                          setMaxApiKeysError(null);
                        }
                      }}
                      onBlur={() => {
                        const numValue = parseInt(maxApiKeys, 10);
                        if (!isNaN(numValue) && numValue >= 1 && numValue <= 100) {
                          updateSystemSetting("max_api_keys_per_user", maxApiKeys);
                        }
                      }}
                      className={cn(
                        "mt-2 max-w-[200px]",
                        maxApiKeysError && "border-destructive"
                      )}
                      aria-invalid={!!maxApiKeysError}
                    />
                    <p className="text-sm text-muted-foreground mt-2">
                      {t("admin.descriptions.max_api_keys_help")}
                    </p>
                    {maxApiKeysError && (
                      <p className="text-sm text-destructive mt-1">
                        {maxApiKeysError}
                      </p>
                    )}
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
                      onClick={handleCreateBackupJob}
                      disabled={creatingBackup}
                      className="w-full"
                    >
                      <Database className="h-4 w-4 mr-2" />
                      {creatingBackup ? t("admin.backup.creating_job") : t("admin.backup.create_backup")}
                    </Button>
                    
                    {restoreUploads.length === 0 ? (
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
                          disabled={analyzingBackup || uploadPhase !== 'idle'}
                          className="w-full"
                        >
                          <Database className="h-4 w-4 mr-2" />
                          {uploadPhase === 'uploading' ? t("admin.loading_states.uploading") : 
                           uploadPhase === 'extracting' ? t("admin.loading_states.extracting") :
                           uploadPhase === 'validating' ? t("admin.loading_states.validating") :
                           t("admin.actions.upload_restore")}
                        </Button>
                        {uploadPhase === 'uploading' && (
                          <Progress
                            value={uploadProgress}
                            durationMs={200}
                            className="mt-2"
                          />
                        )}
                      </div>
                    ) : (
                      <Button
                        variant="default"
                        size="lg"
                        onClick={() => handleRestoreUpload(restoreUploads[0])}
                        disabled={restoringBackup}
                        className="w-full"
                      >
                        <Database className="h-4 w-4 mr-2" />
                        {restoringBackup ? t("admin.loading_states.restoring") : t("admin.actions.restore_backup")}
                      </Button>
                    )}
                  </div>
                  
                  {backupJobs.length > 0 && (
                    <div className="mt-6">
                      <h4 className="text-sm font-medium mb-3">{t("admin.jobs.recent_jobs")}</h4>
                      <div className="space-y-2">
                        {backupJobs
                          .sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime())
                          .slice(0, 8)
                          .map((job) => (
                            <div key={job.id} className="flex items-center justify-between p-3 border rounded-lg">
                              <div className="text-sm flex-1 min-w-0">
                                <div className="font-medium">
                                  {formatDate(job.created_at)}
                                </div>
                                <div className="text-muted-foreground">
                                  {job.status === "failed" ? t("admin.backup.status.failed") :
                                   job.file_size ? formatFileSize(job.file_size) : 
                                   (job.status === "running" || job.status === "pending") ? 
                                   (job.status === "pending" ? t("admin.backup.status.pending") : t("admin.backup.status.running")) : 
                                   null}
                                </div>
                                {job.error_message && (
                                  <div className="text-xs text-destructive mt-1 truncate">
                                    {job.error_message}
                                  </div>
                                )}
                              </div>
                              {getBackupStatusButton(job)}
                            </div>
                          ))}
                      </div>
                    </div>
                  )}

                  <div className="text-sm text-muted-foreground space-y-2">
                    <p>
                      <strong>{t("admin.backup.create_backup")}:</strong> {t("admin.backup.background_description")}
                    </p>
                    <p>
                      <strong>{t("admin.actions.upload_restore")}:</strong> {t("admin.descriptions.upload_restore_description")}
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

              {versionInfo && (
                <div className="flex justify-center mt-8 pt-4 border-t">
                  <div className="text-center text-sm text-muted-foreground">
                    <span>Aviary {versionInfo.version} • </span>
                    <a 
                      href="https://github.com/rmitchellscott/aviary" 
                      target="_blank" 
                      rel="noopener noreferrer"
                      className="text-muted-foreground hover:underline"
                    >
                      GitHub
                    </a>
                  </div>
                </div>
              )}
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
                  if (e.key === 'Enter' && newPasswordValue.length >= 8 && !resettingPassword) {
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
              disabled={resettingPassword || newPasswordValue.length < 8}
            >
              {resettingPassword ? t("admin.loading_states.resetting") : t("admin.actions.reset_password")}
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
                <strong>{restoreConfirmDialog.upload?.filename}</strong>
              </p>
              {backupCounts && (
                <div className="bg-muted p-3 rounded-md">
                  <div className="grid grid-cols-3 gap-4 text-sm mb-4">
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
                  <div className="grid grid-cols-2 gap-4 text-sm">
                    <div className="text-center">
                      <div className="font-semibold text-lg">
                        {backupVersion ? backupVersion.replace(/^v/, '') : t("admin.dialogs.version_unknown")}
                      </div>
                      <div className="text-muted-foreground">{t("admin.dialogs.backup_version")}</div>
                    </div>
                    <div className="text-center">
                      <div className="font-semibold text-lg">
                        {versionInfo?.version ? versionInfo.version.replace(/^v/, '') : t("admin.dialogs.version_unknown")}
                      </div>
                      <div className="text-muted-foreground">{t("admin.dialogs.current_version")}</div>
                    </div>
                  </div>
                </div>
              )}
              
              {(() => {
                const comparison = compareSemver(backupVersion, versionInfo?.version);
                if (comparison === 1) {
                  return (
                    <div className="bg-destructive/10 border border-destructive/20 p-3 rounded-md">
                      <div className="flex items-start gap-2">
                        <AlertTriangle className="h-4 w-4 text-destructive mt-0.5 flex-shrink-0" />
                        <p className="text-sm text-destructive font-medium">
                          {t("admin.dialogs.version_warning")}
                        </p>
                      </div>
                    </div>
                  );
                }
                return null;
              })()}
              
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
                <li>{t("admin.dialogs.restore_warning_items.backups")}</li>
              </ul>
              <p className="text-destructive font-medium">
                {t("admin.dialogs.restore_final_warning")}
              </p>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel 
              disabled={restoringBackup}
              onClick={cancelRestoreUpload}
            >
              {t("admin.actions.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDatabaseRestore}
              disabled={restoringBackup}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {restoringBackup ? t("admin.loading_states.restoring") : t("admin.actions.restore_database")}
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
                <strong>{t('admin.labels.user_id')}:</strong> {viewUser.id}
              </p>
              <p>
                <strong>{t('admin.labels.role')}:</strong>{' '}
                {viewUser.is_admin ? t('admin.roles.admin') : t('admin.roles.user')}
              </p>
              <p>
                <strong>{t('admin.labels.status')}:</strong>{' '}
                {!viewUser.is_active
                  ? t('admin.status.inactive')
                  : viewUser.rmapi_paired
                  ? t('admin.status.paired')
                  : t('admin.status.unpaired')}
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

      {/* Delete Backup Confirmation Dialog */}
      <AlertDialog
        open={deleteBackupDialog.isOpen}
        onOpenChange={closeDeleteBackupDialog}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle className="flex items-center gap-2">
              <AlertTriangle className="h-5 w-5 text-muted-foreground" />
              {t("admin.backup.delete_backup_title")}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t("settings.dialogs.cannot_undo")}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>
              {t("admin.actions.cancel")}
            </AlertDialogCancel>
            <AlertDialogAction
              onClick={confirmDeleteBackup}
              disabled={deleting}
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
            >
              {deleting ? t("admin.loading_states.deleting") : t("admin.actions.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

    </Dialog>
  );
}
