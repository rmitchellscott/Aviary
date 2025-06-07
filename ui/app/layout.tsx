import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Aviary",
  description: "Send documents to reMarkable",
  appleWebApp: {
    title: "Aviary",
  },
};

import { ThemeProvider } from "@/components/theme-provider";
import ThemeSwitcher from "@/components/ThemeSwitcher";
import { Logo } from "@/components/Logo";
import { AuthProvider } from "@/components/AuthProvider";
import { LogoutButton } from "@/components/LogoutButton";

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <script
          dangerouslySetInnerHTML={{
            __html: `
              (function() {
                const stored = localStorage.getItem('theme');
                const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
                const theme = stored === 'light' || stored === 'dark'
                  ? stored
                  : (prefersDark ? 'dark' : 'light');
                if (theme === 'dark') {
                  document.documentElement.classList.add('dark');
                }

                try {
                  const ac = localStorage.getItem('authConfigured');
                  if (ac === 'true') {
                    document.documentElement.classList.add('auth-check');
                  } else if (ac === null) {
                    var xhr = new XMLHttpRequest();
                    xhr.open('GET', '/api/config', false);
                    xhr.send(null);
                    if (xhr.status >= 200 && xhr.status < 400) {
                      var cfg = JSON.parse(xhr.responseText);
                      if (cfg.authEnabled) {
                        localStorage.setItem('authConfigured', 'true');
                        document.documentElement.classList.add('auth-check');
                      } else {
                        localStorage.setItem('authConfigured', 'false');
                      }
                    }
                  }
                } catch (e) {}
              })();
            `,
          }}
        />
      </head>
      <body
        className={`${geistSans.variable} ${geistMono.variable} antialiased`}
      >
        <ThemeProvider
          attribute="class"
          defaultTheme="system"
          enableSystem
          disableTransitionOnChange
        >
        <AuthProvider>
          <header className="flex items-center justify-between px-8 py-2 bg-background">
            <Logo className="h-16 w-32 text-foreground dark:text-foreground-dark" />
            <div className="flex items-center gap-4">
            <LogoutButton />
            <ThemeSwitcher size={24} />
            </div>
          </header>

          <main>{children}</main>
          </AuthProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
