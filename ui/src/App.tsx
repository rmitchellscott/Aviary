import React, { useState } from 'react';
import HomePage from './HomePage';
import { ThemeProvider } from '@/components/theme-provider';
import ThemeSwitcher from '@/components/ThemeSwitcher';
import LanguageSwitcher from '@/components/LanguageSwitcher';
import { Logo } from '@/components/Logo';
import { AuthProvider, useAuth } from '@/components/AuthProvider';
import { LogoutButton } from '@/components/LogoutButton';
import { UserSettings } from '@/components/UserSettings';
import { AdminPanel } from '@/components/AdminPanel';
import { PasswordReset } from '@/components/PasswordReset';
import { Button } from '@/components/ui/button';
import { Settings, Shield, RotateCcw } from 'lucide-react';

function AppContent() {
  const { isAuthenticated, multiUserMode, user } = useAuth();
  const [showUserSettings, setShowUserSettings] = useState(false);
  const [showAdminPanel, setShowAdminPanel] = useState(false);
  const [showPasswordReset, setShowPasswordReset] = useState(false);
  
  // Check if we're on a password reset page
  const isPasswordResetPage = window.location.pathname === '/reset-password' || window.location.search.includes('token=');

  if (isPasswordResetPage) {
    return <PasswordReset onBack={() => window.location.href = '/'} />;
  }

  return (
    <>
      <header className="flex items-center justify-between px-8 py-2 bg-background">
        <Logo className="h-16 w-32 text-foreground dark:text-foreground-dark" />
        <div className="flex items-center gap-4">
          {isAuthenticated && multiUserMode && (
            <>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => setShowUserSettings(true)}
                className="flex items-center gap-2"
              >
                <Settings className="h-4 w-4" />
                Settings
              </Button>
              
              {user?.is_admin && (
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowAdminPanel(true)}
                  className="flex items-center gap-2"
                >
                  <Shield className="h-4 w-4" />
                  Admin
                </Button>
              )}
            </>
          )}
          <LogoutButton />
          <LanguageSwitcher />
          <ThemeSwitcher size={24} />
        </div>
      </header>
      <main>
        <HomePage />
      </main>
      
      {/* Modals */}
      <UserSettings 
        isOpen={showUserSettings} 
        onClose={() => setShowUserSettings(false)} 
      />
      <AdminPanel 
        isOpen={showAdminPanel} 
        onClose={() => setShowAdminPanel(false)} 
      />
    </>
  );
}

export default function App() {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
      <AuthProvider>
        <AppContent />
      </AuthProvider>
    </ThemeProvider>
  );
}
