'use client'

import { useAuth } from './AuthProvider'
import { Button } from './ui/button'
import { useTranslation } from 'react-i18next'

export function LogoutButton() {
  const { isAuthenticated, authConfigured, logout } = useAuth()
  const { t } = useTranslation()

  // Only show logout button if auth is configured and user is authenticated
  if (!authConfigured || !isAuthenticated) {
    return null
  }

  return (
    <Button variant="ghost" size="sm" onClick={logout}>
      {t('logout')}
    </Button>
  )
}
