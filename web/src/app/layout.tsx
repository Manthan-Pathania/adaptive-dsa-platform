import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { Suspense } from "react";
import "./globals.css";
import { AuthProvider } from "@/lib/auth-context";
import { Sidebar } from "@/components/Sidebar";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "Adaptive DSA - FAANG Mastery Engine",
  description: "AI-powered adaptive learning engine for mastering data structures & algorithms interviews with spaced repetition and progressive hint tutoring.",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body
        className={`${geistSans.variable} ${geistMono.variable} min-h-screen bg-[#060709] text-white flex flex-col font-sans antialiased relative selection:bg-[#f59e0b] selection:text-black`}
      >
        <AuthProvider>
          <div className="flex min-h-screen w-full bg-[#060709] relative overflow-x-hidden">
            {/* Ambient Glassmorphic Background System */}
            <div className="pointer-events-none fixed inset-0 overflow-hidden z-0">
              {/* Subtle Refraction Grid Pattern */}
              <div className="absolute inset-0 bg-grid-pattern opacity-40" />

              {/* Top-Right Soft Amber Aurora Orb */}
              <div className="absolute -top-40 right-[-60px] w-[750px] h-[750px] bg-gradient-to-bl from-amber-500/16 via-amber-600/8 to-transparent rounded-full blur-[140px]" />

              {/* Mid-Left Gentle Warm Ambient Orb */}
              <div className="absolute top-[35%] -left-[100px] w-[550px] h-[550px] bg-gradient-to-tr from-amber-600/9 via-amber-700/3 to-transparent rounded-full blur-[160px]" />

              {/* Bottom-Right Deep Twilight Contrast Orb */}
              <div className="absolute -bottom-32 right-[10%] w-[500px] h-[500px] bg-indigo-950/20 rounded-full blur-[160px]" />

              {/* Fluid Glass Refraction Rays */}
              <svg
                className="absolute top-0 right-0 w-[850px] h-[520px] opacity-30"
                viewBox="0 0 850 520"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path
                  d="M100 -50 C 320 140, 580 180, 900 45"
                  stroke="url(#amber-glow-1)"
                  strokeWidth="1.5"
                />
                <path
                  d="M240 -80 C 460 210, 660 240, 930 140"
                  stroke="url(#amber-glow-2)"
                  strokeWidth="1"
                />
                <defs>
                  <linearGradient id="amber-glow-1" x1="0" y1="0" x2="1" y2="0">
                    <stop offset="0%" stopColor="#f59e0b" stopOpacity="0" />
                    <stop offset="50%" stopColor="#f59e0b" stopOpacity="0.65" />
                    <stop offset="100%" stopColor="#fbbf24" stopOpacity="0" />
                  </linearGradient>
                  <linearGradient id="amber-glow-2" x1="0" y1="0" x2="1" y2="0">
                    <stop offset="0%" stopColor="#d97706" stopOpacity="0" />
                    <stop offset="60%" stopColor="#fbbf24" stopOpacity="0.4" />
                    <stop offset="100%" stopColor="#f59e0b" stopOpacity="0" />
                  </linearGradient>
                </defs>
              </svg>
            </div>

            <Suspense fallback={<div className="hidden md:block w-64 border-r border-white/[0.07] bg-[#080a0e]/80 backdrop-blur-2xl" />}>
              <Sidebar />
            </Suspense>
            <div className="flex-1 flex flex-col min-w-0 md:pl-64 z-10">
              <Suspense
                fallback={
                  <div className="flex-1 flex items-center justify-center p-8 text-amber-400/70 font-mono text-xs">
                    Loading workspace...
                  </div>
                }
              >
                <main className="flex-1 flex flex-col">{children}</main>
              </Suspense>
            </div>
          </div>
        </AuthProvider>
      </body>
    </html>
  );
}
