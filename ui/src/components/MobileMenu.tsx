import React from 'react'
import { useTranslation } from 'react-i18next'
import { Settings, Shield, Menu as MenuIcon } from 'lucide-react'
import { Popover, PopoverTrigger, PopoverContent } from '@/components/ui/popover'
import { Button } from '@/components/ui/button'
import ThemeSwitcher from '@/components/ThemeSwitcher'
import LanguageSwitcher from '@/components/LanguageSwitcher'
import { LogoutButton } from '@/components/LogoutButton'
import { useAuth } from '@/components/AuthProvider'

interface MobileMenuProps {
  showSettings?: boolean
  showAdmin?: boolean
  onOpenSettings: () => void
  onOpenAdmin: () => void
}

export function MobileMenu({
  showSettings,
  showAdmin,
  onOpenSettings,
  onOpenAdmin,
}: MobileMenuProps) {
  const { t } = useTranslation()
  const { multiUserMode } = useAuth()

  // If multi-user mode is disabled, show simple controls without hamburger
  if (!multiUserMode) {
    return (
      <div className="flex items-center gap-2">
        <LogoutButton />
        <LanguageSwitcher />
        <ThemeSwitcher size={24} />
      </div>
    )
  }

  // Multi-user mode enabled, show hamburger menu
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="icon" aria-label="Menu">
          <MenuIcon className="h-5 w-5" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="flex flex-col gap-2 w-48">
        {showSettings && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onOpenSettings}
            className="justify-start gap-2"
          >
            <Settings className="h-4 w-4" />
            {t('app.settings')}
          </Button>
        )}
        {showAdmin && (
          <Button
            variant="ghost"
            size="sm"
            onClick={onOpenAdmin}
            className="justify-start gap-2"
          >
            <Shield className="h-4 w-4" />
            {t('app.admin')}
          </Button>
        )}
        <LogoutButton className="justify-start" />
        <div className="flex items-center gap-2">
          <LanguageSwitcher />
          <ThemeSwitcher size={24} />
        </div>
      </PopoverContent>
    </Popover>
  )
}
