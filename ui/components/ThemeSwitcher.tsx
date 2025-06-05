'use client';

import React from 'react';
import { useTheme } from 'next-themes';

interface IconsMap {
  system: React.ReactNode;
  light: React.ReactNode;
  dark: React.ReactNode;
}

interface ThemeSwitcherProps {
  icons?: IconsMap;
  position?: { top: string; right: string };
  size?: number;
}

export default function ThemeSwitcher({
  icons = {
    system: (
      <svg
        xmlns="http://www.w3.org/2000/svg"
        viewBox="0 0 24 24"
        fill="currentColor"                      // keep currentColor
        className="text-muted-foreground"       // apply Tailwind muted color
        style={{ width: '1em', height: '1em' }} // 1em→scales with fontSize
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
  position = { top: '1rem', right: '1rem' },
  size = 24,
}: ThemeSwitcherProps) {
  const { theme, setTheme, systemTheme } = useTheme();

  type Mode = 'system' | 'dark' | 'light';
  const modes: Mode[] = ['system', 'dark', 'light'];

  let currentIndex = -1;
  if (theme === 'system') currentIndex = 0;
  else if (theme === 'dark') currentIndex = 1;
  else if (theme === 'light') currentIndex = 2;

  if (!theme || !systemTheme) return null;

  const handleClick = () => {
    const idx = currentIndex < 0 ? 0 : currentIndex;
    const nextIndex = (idx + 1) % modes.length;
    setTheme(modes[nextIndex]);
  };

  const iconToShow =
    theme === 'system'
      ? icons.system
      : theme === 'dark'
      ? icons.dark
      : icons.light;

  return (
    <button
      onClick={handleClick}
      aria-label={`Switch theme (current: ${
        theme === 'system' ? `system → ${systemTheme}` : theme
      })`}
      style={{
        position: 'fixed',
        background: 'none',
        border: 'none',
        padding: 0,
        cursor: 'pointer',
        top: position.top,
        right: position.right,
        zIndex: 9999,
        fontSize: size,         // controls the SVG’s 1em size
        lineHeight: 0,          // prevent vertical misalignment
      }}
    >
      {iconToShow}
    </button>
  );
}
