import React from 'react';
import HomePage from './HomePage';
import { ThemeProvider } from '@/components/theme-provider';
import ThemeSwitcher from '@/components/ThemeSwitcher';
import LanguageSwitcher from '@/components/LanguageSwitcher';
import { Logo } from '@/components/Logo';
import { AuthProvider } from '@/components/AuthProvider';
import { LogoutButton } from '@/components/LogoutButton';

export default function App() {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
      <AuthProvider>
        <header className="flex items-center justify-between px-8 py-2 bg-background">
          <Logo className="h-16 w-32 text-foreground dark:text-foreground-dark" />
          <div className="flex items-center gap-4">
            <LogoutButton />
            <LanguageSwitcher />
            <ThemeSwitcher size={24} />
          </div>
        </header>
        <main>
          <HomePage />
        </main>
      </AuthProvider>
    </ThemeProvider>
  );
}
