"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { Sparkles, Brain, BookOpen, Clock, ShieldAlert, LogOut, Coins, Compass } from "lucide-react";

export function Navbar() {
  const { user, credits, logout } = useAuth();
  const pathname = usePathname();

  const navLinks = [
    { href: "/dashboard", label: "Dashboard", icon: Brain },
    { href: "/roadmaps", label: "Roadmaps", icon: Compass },
    { href: "/questions", label: "Catalog", icon: BookOpen },
    { href: "/revisions", label: "Revisions", icon: Clock },
    { href: "/credits", label: "Credits", icon: Coins },
  ];

  if (user?.role === "ADMIN") {
    navLinks.push({ href: "/admin", label: "Admin", icon: ShieldAlert });
  }

  return (
    <header className="sticky top-0 z-50 w-full border-b border-slate-800/80 bg-slate-950/80 backdrop-blur-md">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 h-16 flex items-center justify-between">
        <div className="flex items-center gap-8">
          <Link href={user ? "/dashboard" : "/"} className="flex items-center gap-2.5 group">
            <div className="h-9 w-9 rounded-lg bg-gradient-to-tr from-emerald-500 via-teal-500 to-indigo-500 flex items-center justify-center shadow-lg shadow-emerald-500/20 group-hover:shadow-emerald-500/40 transition-all">
              <Brain className="h-5 w-5 text-slate-950 font-bold" />
            </div>
            <div className="flex flex-col">
              <span className="font-bold text-lg tracking-tight bg-gradient-to-r from-white via-slate-200 to-slate-400 bg-clip-text text-transparent">
                Adaptive<span className="text-emerald-400">DSA</span>
              </span>
              <span className="text-[10px] tracking-wider uppercase text-slate-500 font-mono -mt-1">
                Interview Engine
              </span>
            </div>
          </Link>

          {user && (
            <nav className="hidden md:flex items-center gap-1">
              {navLinks.map((link) => {
                const Icon = link.icon;
                const isActive = pathname === link.href || (link.href !== "/dashboard" && pathname.startsWith(link.href));
                return (
                  <Link
                    key={link.href}
                    href={link.href}
                    className={`flex items-center gap-2 px-3 py-1.5 rounded-md text-sm font-medium transition-colors ${
                      isActive
                        ? "bg-slate-800/90 text-emerald-400 border border-slate-700/60 shadow-sm"
                        : "text-slate-400 hover:text-white hover:bg-slate-850"
                    }`}
                  >
                    <Icon className="h-4 w-4" />
                    {link.label}
                  </Link>
                );
              })}
            </nav>
          )}
        </div>

        <div className="flex items-center gap-3">
          {user ? (
            <>
              <Link
                href="/credits"
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-slate-900 border border-emerald-500/30 hover:border-emerald-500/60 transition-all text-xs font-medium text-slate-200 shadow-sm group"
                title="Available AI Credits"
              >
                <Sparkles className="h-3.5 w-3.5 text-emerald-400 animate-pulse" />
                <span className="font-mono font-semibold text-emerald-400">{credits}</span>
                <span className="text-slate-400 text-[11px]">credits</span>
              </Link>

              <div className="hidden sm:flex flex-col items-end text-xs">
                <span className="font-medium text-slate-200">{user.full_name}</span>
                <span className="text-[10px] font-mono text-slate-400 uppercase">{user.role}</span>
              </div>

              <button
                onClick={() => logout()}
                className="p-1.5 rounded-lg text-slate-400 hover:text-rose-400 hover:bg-slate-800 transition-colors"
                title="Sign out"
              >
                <LogOut className="h-4 w-4" />
              </button>
            </>
          ) : (
            <div className="flex items-center gap-2">
              <Link
                href="/login"
                className="px-3.5 py-1.5 text-sm font-medium text-slate-300 hover:text-white transition-colors"
              >
                Sign in
              </Link>
              <Link
                href="/register"
                className="px-3.5 py-1.5 text-sm font-medium text-slate-950 bg-emerald-400 hover:bg-emerald-300 rounded-md transition-colors shadow-sm font-semibold"
              >
                Get Started
              </Link>
            </div>
          )}
        </div>
      </div>
    </header>
  );
}
