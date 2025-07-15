import React from 'react';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { AlertTriangle, Trash2 } from 'lucide-react';

interface User {
  id: string;
  username: string;
  email: string;
  is_admin: boolean;
  is_active: boolean;
}

interface UserDeleteDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => void;
  user: User | null;
  isCurrentUser?: boolean;
  loading?: boolean;
}

export function UserDeleteDialog({ 
  isOpen, 
  onClose, 
  onConfirm, 
  user, 
  isCurrentUser = false,
  loading = false
}: UserDeleteDialogProps) {
  if (!user) return null;

  return (
    <AlertDialog open={isOpen} onOpenChange={onClose}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-red-500" />
            Delete Account
          </AlertDialogTitle>
          <AlertDialogDescription>
            {isCurrentUser ? (
              <>
                Are you sure you want to permanently delete your account?
                <br />
                <br />
                This action will:
                <ul className="list-disc list-inside mt-2 space-y-1">
                  <li>Delete all your data including uploaded documents and API keys</li>
                  <li>Remove all your sessions and log you out</li>
                  <li>Remove all your login history</li>
                  <li>Permanently remove your account from the system</li>
                </ul>
                <br />
                <div className="bg-muted/50 p-3 rounded-md border mb-4">
                  <p className="text-sm text-muted-foreground">
                    <strong>Note:</strong> Your reMarkable Cloud account and device data will remain completely unaffected.
                  </p>
                </div>
                <br />
                <strong className="text-red-600">
                  This action cannot be undone and you will lose access to all your data.
                </strong>
              </>
            ) : (
              <>
                Are you sure you want to permanently delete user <strong>{user.username}</strong>?
                <br />
                <br />
                This action will:
                <ul className="list-disc list-inside mt-2 space-y-1">
                  <li>Delete all user data including archived documents and API keys</li>
                  <li>Remove all user sessions</li>
                  <li>Remove all login history</li>
                </ul>
                <br />
                <div className="bg-muted/50 p-3 rounded-md border mb-4">
                  <p className="text-sm text-muted-foreground">
                    <strong>Note:</strong> The user's reMarkable Cloud account and device data will remain completely unaffected.
                  </p>
                </div>
                <br />
                <strong className="text-red-600">This action cannot be undone.</strong>
              </>
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={onClose} disabled={loading}>
            Cancel
          </AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            disabled={loading}
            className="bg-red-600 hover:bg-red-700 focus:ring-red-600"
          >
            <Trash2 className="h-4 w-4 mr-2" />
            {loading ? 'Deleting...' : isCurrentUser ? 'Delete My Account' : 'Delete User'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
