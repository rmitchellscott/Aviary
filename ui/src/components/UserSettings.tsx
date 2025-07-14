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
  Settings, 
  Key, 
  User, 
  Save, 
  Plus, 
  Trash2, 
  Copy, 
  Eye, 
  EyeOff,
  CheckCircle,
  XCircle,
  Clock
} from 'lucide-react';

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
  const [user, setUser] = useState<User | null>(null);
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  
  // Profile form
  const [email, setEmail] = useState('');
  const [rmapiHost, setRmapiHost] = useState('');
  const [defaultRmdir, setDefaultRmdir] = useState('/');
  
  // Password form
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  
  // API key form
  const [newKeyName, setNewKeyName] = useState('');
  const [newKeyExpiry, setNewKeyExpiry] = useState('');
  const [showNewKey, setShowNewKey] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      fetchUserData();
      fetchAPIKeys();
    }
  }, [isOpen]);

  const fetchUserData = async () => {
    try {
      setLoading(true);
      const response = await fetch('/api/auth/user', {
        credentials: 'include',
      });
      
      if (response.ok) {
        const userData = await response.json();
        setUser(userData);
        setEmail(userData.email);
        
        // Default to environment RMAPI_HOST if user hasn't set their own
        if (userData.rmapi_host) {
          setRmapiHost(userData.rmapi_host);
        } else {
          // Fetch default RMAPI_HOST from system
          try {
            const configResponse = await fetch('/api/config', {
              credentials: 'include',
            });
            if (configResponse.ok) {
              const config = await configResponse.json();
              setRmapiHost(config.rmapi_host || '');
            }
          } catch {
            setRmapiHost('');
          }
        }
        
        setDefaultRmdir(userData.default_rmdir || '/');
      }
    } catch (error) {
      console.error('Failed to fetch user data:', error);
      setError('Failed to load user data');
    } finally {
      setLoading(false);
    }
  };

  const fetchAPIKeys = async () => {
    try {
      const response = await fetch('/api/api-keys', {
        credentials: 'include',
      });
      
      if (response.ok) {
        const keys = await response.json();
        setApiKeys(keys);
      }
    } catch (error) {
      console.error('Failed to fetch API keys:', error);
    }
  };

  const updateProfile = async () => {
    try {
      setSaving(true);
      setError(null);
      
      const response = await fetch('/api/profile', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify({
          email,
          rmapi_host: rmapiHost,
          default_rmdir: defaultRmdir,
        }),
      });

      if (response.ok) {
        await fetchUserData(); // Refresh user data
      } else {
        const errorData = await response.json();
        setError(errorData.error || 'Failed to update profile');
      }
    } catch (error) {
      setError('Failed to update profile');
    } finally {
      setSaving(false);
    }
  };

  const updatePassword = async () => {
    if (newPassword !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }

    try {
      setSaving(true);
      setError(null);
      
      const response = await fetch('/api/profile/password', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify({
          current_password: currentPassword,
          new_password: newPassword,
        }),
      });

      if (response.ok) {
        setCurrentPassword('');
        setNewPassword('');
        setConfirmPassword('');
      } else {
        const errorData = await response.json();
        setError(errorData.error || 'Failed to update password');
      }
    } catch (error) {
      setError('Failed to update password');
    } finally {
      setSaving(false);
    }
  };

  const createAPIKey = async () => {
    try {
      setSaving(true);
      setError(null);

      const body: any = { name: newKeyName };
      if (newKeyExpiry) {
        const expiryDate = new Date(newKeyExpiry);
        body.expires_at = Math.floor(expiryDate.getTime() / 1000);
      }
      
      const response = await fetch('/api/api-keys', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        credentials: 'include',
        body: JSON.stringify(body),
      });

      if (response.ok) {
        const newKey = await response.json();
        setShowNewKey(newKey.api_key);
        setNewKeyName('');
        setNewKeyExpiry('');
        await fetchAPIKeys();
      } else {
        const errorData = await response.json();
        setError(errorData.error || 'Failed to create API key');
      }
    } catch (error) {
      setError('Failed to create API key');
    } finally {
      setSaving(false);
    }
  };

  const deleteAPIKey = async (keyId: string) => {
    try {
      const response = await fetch(`/api/api-keys/${keyId}`, {
        method: 'DELETE',
        credentials: 'include',
      });

      if (response.ok) {
        await fetchAPIKeys();
      }
    } catch (error) {
      console.error('Failed to delete API key:', error);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
  };

  const formatDate = (dateString: string) => {
    return new Date(dateString).toLocaleDateString();
  };

  const getKeyStatus = (key: APIKey) => {
    if (!key.is_active) return 'inactive';
    if (key.expires_at && new Date(key.expires_at) < new Date()) return 'expired';
    return 'active';
  };

  if (loading) {
    return (
      <Dialog open={isOpen} onOpenChange={onClose}>
        <DialogContent className="max-w-4xl">
          <div className="flex items-center justify-center p-8">
            Loading...
          </div>
        </DialogContent>
      </Dialog>
    );
  }

  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="max-w-4xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Settings className="h-5 w-5" />
            User Settings
          </DialogTitle>
          <DialogDescription>
            Manage your profile, API keys, and preferences
          </DialogDescription>
        </DialogHeader>

        {error && (
          <div className="bg-destructive/10 border border-destructive/20 rounded-md p-3 text-destructive">
            {error}
          </div>
        )}

        <Tabs defaultValue="profile" className="w-full">
          <TabsList className="grid w-full grid-cols-3">
            <TabsTrigger value="profile" className="flex items-center gap-2">
              <User className="h-4 w-4" />
              Profile
            </TabsTrigger>
            <TabsTrigger value="password">Password</TabsTrigger>
            <TabsTrigger value="api-keys" className="flex items-center gap-2">
              <Key className="h-4 w-4" />
              API Keys
            </TabsTrigger>
          </TabsList>

          <TabsContent value="profile">
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
                      value={user?.username || ''} 
                      disabled 
                    />
                  </div>
                  <div>
                    <Label htmlFor="email">Email</Label>
                    <Input
                      id="email"
                      type="email"
                      value={email}
                      onChange={(e) => setEmail(e.target.value)}
                    />
                  </div>
                </div>
                
                <div>
                  <Label htmlFor="rmapi-host">reMarkable Host (optional)</Label>
                  <Input
                    id="rmapi-host"
                    value={rmapiHost}
                    onChange={(e) => setRmapiHost(e.target.value)}
                    placeholder="Leave empty for reMarkable Cloud"
                  />
                </div>
                
                <div>
                  <Label htmlFor="default-rmdir">Default Directory</Label>
                  <Input
                    id="default-rmdir"
                    value={defaultRmdir}
                    onChange={(e) => setDefaultRmdir(e.target.value)}
                    placeholder="/"
                  />
                </div>

                <div className="flex justify-end">
                  <Button onClick={updateProfile} disabled={saving}>
                    <Save className="h-4 w-4 mr-2" />
                    {saving ? 'Saving...' : 'Save Changes'}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="password">
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
                  />
                </div>
                
                <div>
                  <Label htmlFor="new-password">New Password</Label>
                  <Input
                    id="new-password"
                    type="password"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                  />
                </div>
                
                <div>
                  <Label htmlFor="confirm-password">Confirm New Password</Label>
                  <Input
                    id="confirm-password"
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                  />
                </div>

                <div className="flex justify-end">
                  <Button 
                    onClick={updatePassword} 
                    disabled={saving || !currentPassword || !newPassword || !confirmPassword}
                  >
                    <Save className="h-4 w-4 mr-2" />
                    {saving ? 'Updating...' : 'Update Password'}
                  </Button>
                </div>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="api-keys">
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
                      />
                    </div>
                    <div>
                      <Label htmlFor="key-expiry">Expiry (optional)</Label>
                      <Select value={newKeyExpiry} onValueChange={setNewKeyExpiry}>
                        <SelectTrigger>
                          <SelectValue placeholder="Never expires" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="">Never</SelectItem>
                          <SelectItem value={new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]}>1 week</SelectItem>
                          <SelectItem value={new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]}>1 month</SelectItem>
                          <SelectItem value={new Date(Date.now() + 90 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]}>3 months</SelectItem>
                          <SelectItem value={new Date(Date.now() + 365 * 24 * 60 * 60 * 1000).toISOString().split('T')[0]}>1 year</SelectItem>
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
                      <code className="flex-1 font-mono text-sm">{showNewKey}</code>
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
                              <TableCell>
                                <Button
                                  size="sm"
                                  variant="destructive"
                                  onClick={() => deleteAPIKey(key.id)}
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
  );
}