import React, { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
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
  Server
} from 'lucide-react';

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
}

interface AdminPanelProps {
  isOpen: boolean;
  onClose: () => void;
}

export function AdminPanel({ isOpen, onClose }: AdminPanelProps) {
  const { t } = useTranslation();
  const [users, setUsers] = useState<User[]>([]);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [systemStatus, setSystemStatus] = useState<SystemStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // User creation form
  const [newUsername, setNewUsername] = useState('');
  const [newEmail, setNewEmail] = useState('');
  const [newPassword, setNewPassword] = useState('');
  
  // Settings
  const [registrationEnabled, setRegistrationEnabled] = useState(false);
  const [maxApiKeys, setMaxApiKeys] = useState('10');
  const [sessionTimeout, setSessionTimeout] = useState('24');

  useEffect(() => {
    if (isOpen) {
      fetchSystemStatus();
      fetchUsers();
      fetchAPIKeys();
    }
  }, [isOpen]);

  const fetchSystemStatus = async () => {
    try {
      const response = await fetch('/api/admin/status', {
        credentials: 'include',
      });
      
      if (response.ok) {
        const status = await response.json();
        setSystemStatus(status);
        setRegistrationEnabled(status.settings.registration_enabled === 'true');
        setMaxApiKeys(status.settings.max_api_keys_per_user);
        setSessionTimeout(status.settings.session_timeout_hours);
      }
    } catch (error) {
      console.error('Failed to fetch system status:', error);
    }
  };

  const fetchUsers = async () => {
    try {
      const response = await fetch('/api/users', {
        credentials: 'include',
      });
      
      if (response.ok) {
        const data = await response.json();
        setUsers(data.users);
      }
    } catch (error) {
      console.error('Failed to fetch users:', error);
    }
  };

  const fetchAPIKeys = async () => {
    try {
      const response = await fetch('/api/admin/api-keys', {
        credentials: 'include',
      });
      
      if (response.ok) {
        const data = await response.json();
        setApiKeys(data.api_keys);
      }
    } catch (error) {
      console.error('Failed to fetch API keys:', error);
    }
  };

  const createUser = async () => {
    try {
      setSaving(true);
      setError(null);
      
      const response = await fetch('/api/auth/register', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify({
          username: newUsername,
          email: newEmail,
          password: newPassword,
        }),
      });

      if (response.ok) {
        setNewUsername('');
        setNewEmail('');
        setNewPassword('');
        await fetchUsers();
        await fetchSystemStatus();
      } else {
        const errorData = await response.json();
        setError(errorData.error || 'Failed to create user');
      }
    } catch (error) {
      setError('Failed to create user');
    } finally {
      setSaving(false);
    }
  };

  const toggleUserStatus = async (userId: string, isActive: boolean) => {
    try {
      const endpoint = isActive ? 'activate' : 'deactivate';
      const response = await fetch(`/api/users/${userId}/${endpoint}`, {
        method: 'POST',
        credentials: 'include',
      });

      if (response.ok) {
        await fetchUsers();
      }
    } catch (error) {
      console.error('Failed to toggle user status:', error);
    }
  };

  const updateSystemSetting = async (key: string, value: string) => {
    try {
      const response = await fetch('/api/admin/settings', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify({ key, value }),
      });

      if (response.ok) {
        await fetchSystemStatus();
      }
    } catch (error) {
      console.error('Failed to update setting:', error);
    }
  };

  const testSMTP = async () => {
    try {
      setSaving(true);
      setError(null);
      
      const response = await fetch('/api/admin/test-smtp', {
        method: 'POST',
        credentials: 'include',
      });

      const result = await response.json();
      if (response.ok) {
        setError(null);
        // Show success message
      } else {
        setError(result.error || 'SMTP test failed');
      }
    } catch (error) {
      setError('Failed to test SMTP');
    } finally {
      setSaving(false);
    }
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString();
  };

  const getKeyStatus = (key: APIKey) => {
    if (!key.is_active) return 'inactive';
    if (key.expires_at && new Date(key.expires_at) < new Date()) return 'expired';
    return 'active';
  };

  const getSMTPStatusColor = (status: string) => {
    switch (status) {
      case 'configured_and_working': return 'success';
      case 'configured_but_failed': return 'destructive';
      default: return 'secondary';
    }
  };

  const getSMTPStatusText = (status: string) => {
    switch (status) {
      case 'configured_and_working': return 'Working';
      case 'configured_but_failed': return 'Failed';
      case 'not_configured': return 'Not configured';
      default: return 'Unknown';
    }
  };

  if (!systemStatus) {
    return (
      <Dialog open={isOpen} onOpenChange={onClose}>
        <DialogContent className="max-w-6xl">
          <div className="flex items-center justify-center p-8">
            Loading...
          </div>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
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
                  <CardTitle className="text-sm font-medium">Total Users</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{systemStatus.database.total_users}</div>
                  <p className="text-xs text-muted-foreground">
                    {systemStatus.database.active_users} active
                  </p>
                </CardContent>
              </Card>
              
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">API Keys</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{systemStatus.database.api_keys.total}</div>
                  <p className="text-xs text-muted-foreground">
                    {systemStatus.database.api_keys.active} active
                  </p>
                </CardContent>
              </Card>
              
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">Documents</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{systemStatus.database.documents}</div>
                  <p className="text-xs text-muted-foreground">Total uploaded</p>
                </CardContent>
              </Card>
              
              <Card>
                <CardHeader className="pb-2">
                  <CardTitle className="text-sm font-medium">Active Sessions</CardTitle>
                </CardHeader>
                <CardContent>
                  <div className="text-2xl font-bold">{systemStatus.database.active_sessions}</div>
                  <p className="text-xs text-muted-foreground">Current sessions</p>
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
                    <Badge variant={getSMTPStatusColor(systemStatus.smtp.status)}>
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
                  <Badge variant="success">Multi-user Mode</Badge>
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
                      />
                    </div>
                  </div>
                  
                  <div className="flex justify-end">
                    <Button 
                      onClick={createUser} 
                      disabled={saving || !newUsername || !newEmail || !newPassword}
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
                            <Badge variant={user.is_admin ? 'default' : 'secondary'}>
                              {user.is_admin ? 'Admin' : 'User'}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            <Badge variant={user.is_active ? 'success' : 'secondary'}>
                              {user.is_active ? 'Active' : 'Inactive'}
                            </Badge>
                          </TableCell>
                          <TableCell>{formatDate(user.created_at)}</TableCell>
                          <TableCell>
                            {user.last_login ? formatDate(user.last_login) : 'Never'}
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2">
                              <Button
                                size="sm"
                                variant={user.is_active ? 'destructive' : 'default'}
                                onClick={() => toggleUserStatus(user.id, !user.is_active)}
                              >
                                {user.is_active ? 'Deactivate' : 'Activate'}
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
                                status === 'active' ? 'success' : 
                                status === 'expired' ? 'destructive' : 
                                'secondary'
                              }
                            >
                              {status === 'active' && <CheckCircle className="h-3 w-3 mr-1" />}
                              {status === 'expired' && <XCircle className="h-3 w-3 mr-1" />}
                              {status === 'inactive' && <Clock className="h-3 w-3 mr-1" />}
                              {status}
                            </Badge>
                          </TableCell>
                          <TableCell>{formatDate(key.created_at)}</TableCell>
                          <TableCell>
                            {key.last_used ? formatDate(key.last_used) : 'Never'}
                          </TableCell>
                          <TableCell>
                            {key.expires_at ? formatDate(key.expires_at) : 'Never'}
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
                      <Label htmlFor="registration-enabled">Enable User Registration</Label>
                      <p className="text-sm text-muted-foreground">
                        Allow admins to create new user accounts
                      </p>
                    </div>
                    <Switch
                      id="registration-enabled"
                      checked={registrationEnabled}
                      onCheckedChange={(checked) => {
                        setRegistrationEnabled(checked);
                        updateSystemSetting('registration_enabled', checked.toString());
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
                    <Label htmlFor="max-api-keys">Maximum API Keys per User</Label>
                    <Input
                      id="max-api-keys"
                      type="number"
                      value={maxApiKeys}
                      onChange={(e) => setMaxApiKeys(e.target.value)}
                      onBlur={() => updateSystemSetting('max_api_keys_per_user', maxApiKeys)}
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
                    <Label htmlFor="session-timeout">Session Timeout (hours)</Label>
                    <Input
                      id="session-timeout"
                      type="number"
                      value={sessionTimeout}
                      onChange={(e) => setSessionTimeout(e.target.value)}
                      onBlur={() => updateSystemSetting('session_timeout_hours', sessionTimeout)}
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
                    <Button variant="outline" disabled>
                      <Database className="h-4 w-4 mr-2" />
                      Backup Database
                    </Button>
                    <Button variant="outline" disabled>
                      <Database className="h-4 w-4 mr-2" />
                      Restore Database
                    </Button>
                  </div>
                  <p className="text-sm text-muted-foreground">
                    Database backup and restore functionality coming soon
                  </p>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Maintenance</CardTitle>
                </CardHeader>
                <CardContent>
                  <Button variant="outline">
                    <Trash2 className="h-4 w-4 mr-2" />
                    Cleanup Old Data
                  </Button>
                </CardContent>
              </Card>
            </div>
          </TabsContent>
        </Tabs>
      </DialogContent>
    </Dialog>
  );
}
