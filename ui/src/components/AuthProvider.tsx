'use client'

import { createContext, useContext, useEffect, useState, ReactNode } from 'react'

interface AuthContextType {
  isAuthenticated: boolean
  isLoading: boolean
  authConfigured: boolean
  uiSecret: string | null
  login: () => Promise<void>
  logout: () => void
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}

interface AuthProviderProps {
  children: ReactNode
}

export function AuthProvider({ children }: AuthProviderProps) {
  const storedConf =
    typeof window !== 'undefined' ? localStorage.getItem('authConfigured') : null
  const initialAuthConfigured = storedConf === 'true'
  const [authConfigured, setAuthConfigured] = useState<boolean>(initialAuthConfigured)
  const [uiSecret, setUiSecret] = useState<string | null>(null)
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(() => {
    if (typeof window !== 'undefined') {
      // Check if we have a UI secret injected (means web auth is disabled)
      const hasUISecret = !!(window as Window & { __UI_SECRET__?: string }).__UI_SECRET__
      if (hasUISecret) {
        // UI secret present - we'll need to call server to get JWT, start as false
        return false
      }
      
      const authConfigured = localStorage.getItem('authConfigured')
      if (authConfigured === 'false') {
        // No web auth configured and no UI secret, start as authenticated
        return true
      }
      
      // Web auth is configured - be more conservative about initial state
      // Only trust localStorage if we recently checked
      const expiry = parseInt(localStorage.getItem('authExpiry') || '0', 10)
      const lastCheck = parseInt(localStorage.getItem('lastAuthCheck') || '0', 10)
      const recentCheck = Date.now() - lastCheck < 5 * 60 * 1000 // 5 minutes
      
      return recentCheck && expiry > Date.now()
    }
    return false
  })
  const [isLoading, setIsLoading] = useState<boolean>(true) // Always start loading

  const checkAuth = async () => {
    try {
      // First check if auth is configured
      const configResponse = await fetch('/api/config')
      const configData = await configResponse.json()
      
      // Get UI secret from window (injected by server when web auth is disabled)
      const uiSecret = (window as Window & { __UI_SECRET__?: string }).__UI_SECRET__ || null
      setUiSecret(uiSecret)
      
      if (configData.authEnabled) {
        // Web authentication is enabled - users need to log in
        setAuthConfigured(true)
        if (typeof window !== 'undefined') {
          localStorage.setItem('authConfigured', 'true')
        }
        const response = await fetch('/api/auth/check', {
          credentials: 'include',
          headers: {
            'X-UI-Token': uiSecret || ''
          }
        })
        const data = await response.json()
        setIsAuthenticated(data.authenticated)
        if (typeof window !== 'undefined') {
          if (data.authenticated) {
            const expiry = Date.now() + 24 * 3600 * 1000
            localStorage.setItem('authExpiry', expiry.toString())
          } else {
            localStorage.setItem('authExpiry', '0')
          }
        }
      } else if (uiSecret) {
        // Web auth is disabled but we have UI secret - call auth check to get auto-JWT
        setAuthConfigured(false)
        if (typeof window !== 'undefined') {
          localStorage.setItem('authConfigured', 'false')
        }
        const response = await fetch('/api/auth/check', {
          credentials: 'include',
          headers: {
            'X-UI-Token': uiSecret
          }
        })
        const data = await response.json()
        setIsAuthenticated(data.authenticated)
        if (typeof window !== 'undefined') {
          const expiry = Date.now() + 24 * 3600 * 1000
          localStorage.setItem('authExpiry', expiry.toString())
        }
      } else if (configData.apiKeyEnabled) {
        // Only API key auth is enabled (for external clients)
        // UI users are automatically authenticated
        setAuthConfigured(false)
        setIsAuthenticated(true)
        if (typeof window !== 'undefined') {
          localStorage.setItem('authConfigured', 'false')
          const expiry = Date.now() + 365 * 24 * 3600 * 1000
          localStorage.setItem('authExpiry', expiry.toString())
        }
      } else {
        // No authentication configured at all
        setAuthConfigured(false)
        setIsAuthenticated(true)
        if (typeof window !== 'undefined') {
          localStorage.setItem('authConfigured', 'false')
          const expiry = Date.now() + 365 * 24 * 3600 * 1000
          localStorage.setItem('authExpiry', expiry.toString())
        }
      }
    } catch {
      // Default to authenticated if we can't check (fail open for UI)
      setAuthConfigured(false)
      setIsAuthenticated(true)
      if (typeof window !== 'undefined') {
        localStorage.setItem('authConfigured', 'false')
        const expiry = Date.now() + 365 * 24 * 3600 * 1000
        localStorage.setItem('authExpiry', expiry.toString())
      }
    } finally {
      setIsLoading(false)
      if (typeof window !== 'undefined') {
        document.documentElement.classList.remove('auth-check')
      }
    }
  }

  const login = async () => {
    // Don't set isAuthenticated immediately
    // Instead, re-check auth status to ensure JWT cookie is set
    await checkAuth()
  }

  const logout = async () => {
    if (!authConfigured) return
    
    try {
      await fetch('/api/auth/logout', { 
        method: 'POST',
        credentials: 'include'
      })
    } catch {
      // Handle error silently
    }
    setIsAuthenticated(false)
    if (typeof window !== 'undefined') {
      localStorage.setItem('authExpiry', '0')
    }
  }

  useEffect(() => {
    checkAuth()
  }, [])

  useEffect(() => {
    if (!isLoading && typeof window !== 'undefined') {
      document.documentElement.classList.remove('auth-check')
    }
  }, [isLoading])

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, login, logout, authConfigured, uiSecret }}>
      {children}
    </AuthContext.Provider>
  )
}
