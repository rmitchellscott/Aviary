'use client';

import React, { useState, useEffect } from 'react';
import { useTheme } from 'next-themes';
import { Button } from '@/components/ui/button';
import { useTranslation } from 'react-i18next';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';

interface IconsMap {
  system: React.ReactNode;
  light: React.ReactNode;
  dark: React.ReactNode;
}

interface ThemeSwitcherProps {
  icons?: IconsMap;
  size?: number;
}

export default function ThemeSwitcher({
  icons = {
    system: (
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="currentColor"
        className="text-muted-foreground"
        style={{ width: '1em', height: '1em' }}
      >
        <path d="M12 21.997c-5.523 0-10-4.477-10-10s4.477-10 10-10s10 4.477 10 10s-4.477 10-10 10m0-2a8 8 0 1 0 0-16a8 8 0 0 0 0 16m0-2v-12a6 6 0 0 1 0 12" />
      </svg>
    ),
    light: (
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="currentColor"
        className="text-muted-foreground"
        style={{
          width: '1em',
          height: '1em',
          transform: 'rotate(-90deg)',
          transformOrigin: '50% 50%',
        }}
      >
        <path d="M12 21.997c-5.523 0-10-4.477-10-10s4.477-10 10-10s10 4.477 10 10s-4.477 10-10 10m0-2a8 8 0 1 0 0-16a8 8 0 0 0 0 16m0-2v-12a6 6 0 0 1 0 12" />
      </svg>
    ),
    dark: (
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="currentColor"
        className="text-muted-foreground"
        style={{ width: '1em', height: '1em' }}
      >
        <path d="M12 21.997c-5.523 0-10-4.477-10-10s4.477-10 10-10s10 4.477 10 10s-4.477 10-10 10m0-2a8 8 0 1 0 0-16a8 8 0 0 0 0 16m-5-4.681a8.965 8.965 0 0 0 5.707-2.612a8.965 8.965 0 0 0 2.612-5.707A6 6 0 1 1 7 15.316" />
      </svg>
    ),
  },
  size = 24,
}: ThemeSwitcherProps) {
  const [mounted, setMounted] = useState(false);
  useEffect(() => {
    setMounted(true);
  }, []);

  const { theme, setTheme, systemTheme } = useTheme();

  type Mode = 'system' | 'dark' | 'light';
  const modes: Mode[] = ['system', 'dark', 'light'];

  let currentIndex = -1;
  if (theme === 'system') currentIndex = 0;
  else if (theme === 'dark') currentIndex = 1;
  else if (theme === 'light') currentIndex = 2;

  const { t } = useTranslation();

  if (!mounted || !theme || !systemTheme) {
    return null;
  }

  const nextIndex = (currentIndex < 0 ? 0 : currentIndex + 1) % modes.length;
  const nextMode = modes[nextIndex];

  const handleClick = () => {
    setTheme(nextMode);
  };

  const iconToShow =
    theme === 'system' ? icons.system : theme === 'dark' ? icons.dark : icons.light;
  const labelMap: Record<Mode, string> = {
    system: t('theme.auto'),
    dark: t('theme.dark'),
    light: t('theme.light'),
  };

  return (
    <Tooltip>
      <TooltipTrigger asChild key={theme}>
        <Button
          variant="ghost"
          size="icon"
          onClick={handleClick}
          style={{
            fontSize: size,
            lineHeight: 0,
          }}
        >
          {iconToShow}
        </Button>
      </TooltipTrigger>
      <TooltipContent>{t('theme.switch', { mode: labelMap[nextMode] })}</TooltipContent>
    </Tooltip>
  );
}
