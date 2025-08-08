'use client'

import { useAuth } from './AuthProvider'
import { Button } from './ui/button'
import { useTranslation } from 'react-i18next'
import { LogOut } from 'lucide-react'

export function LogoutButton({ className = '', iconOnly = false }: { className?: string, iconOnly?: boolean }) {
  const { isAuthenticated, authConfigured, logout } = useAuth()
  const { t } = useTranslation()

  // Only show logout button if auth is configured and user is authenticated
  if (!authConfigured || !isAuthenticated) {
    return null
  }

  return (
    <Button variant="ghost" size="sm" onClick={logout} className={`flex items-center gap-2 ${className}`}>
      <LogOut className="h-4 w-4" />
      {!iconOnly && <span className="sm:hidden lg:inline">{t('logout')}</span>}
    </Button>
  )
}
