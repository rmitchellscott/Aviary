'use client'

import { useAuth } from './AuthProvider'
import { Button } from './ui/button'

export function LogoutButton() {
  const { isAuthenticated, logout } = useAuth()

  // Only show logout button if user is authenticated
  if (!isAuthenticated) {
    return null
  }

  return (
    <Button variant="ghost" size="sm" onClick={logout}>
      Logout
    </Button>
  )
}
