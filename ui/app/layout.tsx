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
