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

import { ThemeProvider } from "@/components/theme-provider"
import ThemeSwitcher from '@/components/ThemeSwitcher';

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
          {/* Place the switcher at the top‐right corner */}
          <ThemeSwitcher
            position={{ top: '1rem', right: '1rem' }}
            size={24}
          />
        {children}
        </ThemeProvider>
      </body>
    </html>
  );
}
