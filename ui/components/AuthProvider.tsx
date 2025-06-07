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
  const [isAuthenticated, setIsAuthenticated] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [authConfigured, setAuthConfigured] = useState(false)

  const checkAuth = async () => {
    try {
      // First check if auth is configured
      const configResponse = await fetch('/api/config')
      const configData = await configResponse.json()
      
      if (configData.authEnabled) {
        setAuthConfigured(true)
        const response = await fetch('/api/auth/check', {
          credentials: 'include'
        })
        const data = await response.json()
        setIsAuthenticated(data.authenticated)
      } else {
        setAuthConfigured(false)
        setIsAuthenticated(true) // No auth required, so consider authenticated
      }
    } catch {
      setAuthConfigured(false)
      setIsAuthenticated(true) // Default to authenticated if we can't check
    } finally {
      setIsLoading(false)
    }
  }

  const login = () => {
    setIsAuthenticated(true)
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
  }

  useEffect(() => {
    checkAuth()
  }, [])

  return (
    <AuthContext.Provider value={{ isAuthenticated, isLoading, login, logout, authConfigured }}>
      {children}
    </AuthContext.Provider>
  )
}
