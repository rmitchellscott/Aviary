'use client'

import { createContext, useContext, useEffect, useState, ReactNode } from 'react'

interface AuthContextType {
  isAuthenticated: boolean
  isLoading: boolean
  authConfigured: boolean
  login: () => void
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
  const [isAuthenticated, setIsAuthenticated] = useState<boolean>(() => {
    if (typeof window !== 'undefined') {
      const expiry = parseInt(localStorage.getItem('authExpiry') || '0', 10)
      return expiry > Date.now()
    }
    return false
  })
  const [isLoading, setIsLoading] = useState<boolean>(initialAuthConfigured)

  const checkAuth = async () => {
    try {
      // First check if auth is configured
      const configResponse = await fetch('/api/config')
      const configData = await configResponse.json()
      
      if (configData.authEnabled) {
        setAuthConfigured(true)
        if (typeof window !== 'undefined') {
          localStorage.setItem('authConfigured', 'true')
        }
        const response = await fetch('/api/auth/check', {
          credentials: 'include'
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
      } else {
        setAuthConfigured(false)
        if (typeof window !== 'undefined') {
          localStorage.setItem('authConfigured', 'false')
          const expiry = Date.now() + 365 * 24 * 3600 * 1000
          localStorage.setItem('authExpiry', expiry.toString())
        }
        setIsAuthenticated(true) // No auth required, so consider authenticated
      }
    } catch {
      setAuthConfigured(false)
      if (typeof window !== 'undefined') {
        localStorage.setItem('authConfigured', 'false')
        const expiry = Date.now() + 365 * 24 * 3600 * 1000
        localStorage.setItem('authExpiry', expiry.toString())
      }
      setIsAuthenticated(true) // Default to authenticated if we can't check
    } finally {
      setIsLoading(false)
      if (typeof window !== 'undefined') {
        document.documentElement.classList.remove('auth-check')
      }
    }
  }

  const login = () => {
    setIsAuthenticated(true)
    if (typeof window !== 'undefined') {
      const expiry = Date.now() + 24 * 3600 * 1000
      localStorage.setItem('authExpiry', expiry.toString())
    }
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
    <AuthContext.Provider value={{ isAuthenticated, isLoading, login, logout, authConfigured }}>
      {children}
    </AuthContext.Provider>
  )
}
