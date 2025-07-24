import React from 'react';
import { useTranslation } from 'react-i18next';
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
  const { t } = useTranslation();
  if (!user) return null;

  return (
    <AlertDialog open={isOpen} onOpenChange={onClose}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle className="flex items-center gap-2">
            <AlertTriangle className="h-5 w-5 text-destructive" />
            {t("user_delete.title")}
          </AlertDialogTitle>
          <AlertDialogDescription>
            {isCurrentUser ? (
              <>
                {t("user_delete.current_user_warning")}
                <br />
                <br />
                {t("user_delete.action_will")}
                <ul className="list-disc list-inside mt-2 space-y-1">
                  <li>{t("user_delete.current_user.data_deletion")}</li>
                  <li>{t("user_delete.current_user.remove_sessions")}</li>
                  <li>{t("user_delete.current_user.remove_history")}</li>
                  <li>{t("user_delete.current_user.permanent_removal")}</li>
                </ul>
                <br />
                <div className="bg-muted/50 p-3 rounded-md border mb-4">
                  <p className="text-sm text-muted-foreground">
                    <strong>{t("user_delete.note_label")}</strong> {t("user_delete.note_current")}
                  </p>
                </div>
                <br />
                <strong className="text-destructive">
                  {t("user_delete.cannot_undo_current")}
                </strong>
              </>
            ) : (
              <>
                <span dangerouslySetInnerHTML={{ __html: t("user_delete.admin_user_warning", { username: user.username }) }} />
                <br />
                <br />
                {t("user_delete.action_will")}
                <ul className="list-disc list-inside mt-2 space-y-1">
                  <li>{t("user_delete.admin_user.data_deletion")}</li>
                  <li>{t("user_delete.admin_user.remove_sessions")}</li>
                  <li>{t("user_delete.admin_user.remove_history")}</li>
                </ul>
                <br />
                <div className="bg-muted/50 p-3 rounded-md border mb-4">
                  <p className="text-sm text-muted-foreground">
                    <strong>{t("user_delete.note_label")}</strong> {t("user_delete.note_admin")}
                  </p>
                </div>
                <br />
                <strong className="text-destructive">{t("user_delete.cannot_undo_admin")}</strong>
              </>
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel onClick={onClose} disabled={loading}>
            {t("user_delete.cancel")}
          </AlertDialogCancel>
          <AlertDialogAction
            onClick={onConfirm}
            disabled={loading}
            className="bg-destructive hover:bg-destructive/90 focus:ring-destructive"
          >
            <Trash2 className="h-4 w-4 mr-2" />
            {loading ? t("user_delete.deleting") : isCurrentUser ? t("user_delete.delete_account") : t("user_delete.delete_user")}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
