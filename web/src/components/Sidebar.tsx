"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { useAuth } from "@/lib/auth-context";
import {
  LayoutGrid,
  Layers,
  Map,
  RotateCw,
  Sparkles,
  Shield,
  LogOut,
  Menu,
  X,
  Code2,
  Coins,
  ChevronRight,
} from "lucide-react";

export function Sidebar() {
  const { user, credits, logout } = useAuth();
  const pathname = usePathname();
  const [mobileOpen, setMobileOpen] = useState(false);

  const navLinks = [
    {
      href: "/dashboard",
      label: "Dashboard",
      icon: LayoutGrid,
    },
    {
      href: "/questions",
      label: "Master Question Bank",
      icon: Layers,
    },
    {
      href: "/roadmaps",
      label: "Roadmaps",
      icon: Map,
    },
    {
      href: "/revisions",
      label: "Spaced Revisions",
      icon: RotateCw,
    },
    {
      href: "/credits",
      label: "AI Wallet",
      icon: Sparkles,
    },
  ];

  if (user?.role === "ADMIN") {
    navLinks.push({
      href: "/admin",
      label: "Admin Catalog",
      icon: Shield,
    });
  }

  const renderNavContent = () => (
    <div className="flex flex-col h-full bg-[#080a0e]/75 backdrop-blur-3xl text-white select-none border-r border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.06)]">
      {/* Brand Header */}
      <div className="p-4 px-5 border-b border-white/[0.07] flex items-center justify-between">
        <Link
          href={user ? "/dashboard" : "/"}
          onClick={() => setMobileOpen(false)}
          className="flex items-center gap-3 group"
        >
          <div className="h-10 w-10 rounded-xl bg-[#121520]/80 border border-amber-500/30 flex items-center justify-center shadow-[0_0_15px_rgba(245,158,11,0.18)] group-hover:scale-105 group-hover:border-amber-400/60 transition-all">
            <Code2 className="h-5 w-5 text-amber-400 stroke-[2.4]" />
          </div>
          <div className="flex flex-col">
            <span className="font-extrabold text-base tracking-wide text-white">
              Adaptive<span className="text-amber-400">DSA</span>
            </span>
            <span className="text-[10px] tracking-wider uppercase text-neutral-400 font-medium -mt-0.5">
              FAANG Mastery Engine
            </span>
          </div>
        </Link>
        <button
          onClick={() => setMobileOpen(false)}
          className="md:hidden p-1.5 rounded-lg text-neutral-400 hover:text-white hover:bg-white/[0.05] transition-colors"
          aria-label="Close navigation"
        >
          <X className="h-5 w-5" />
        </button>
      </div>

      {/* AI Balance Card with Frosted Glass */}
      {user && (
        <div className="mx-4 mt-4">
          <Link
            href="/credits"
            onClick={() => setMobileOpen(false)}
            className="p-3 rounded-xl bg-[#10131d]/60 border border-white/[0.09] backdrop-blur-xl hover:border-amber-500/40 hover:bg-[#141824]/70 shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)] transition-all flex items-center justify-between group block"
          >
            <div className="flex items-center gap-3">
              <div className="p-2 rounded-lg bg-amber-500/10 border border-amber-500/20 text-amber-400">
                <Coins className="h-4 w-4 stroke-[2.2]" />
              </div>
              <div className="flex flex-col">
                <span className="text-[11px] font-medium text-neutral-400">
                  AI Balance
                </span>
                <span className="text-sm font-bold text-white group-hover:text-amber-400 transition-colors">
                  {credits} Credits
                </span>
              </div>
            </div>
            <ChevronRight className="h-4 w-4 text-neutral-500 group-hover:text-white group-hover:translate-x-0.5 transition-all" />
          </Link>
        </div>
      )}

      {/* Main Navigation Links */}
      <nav className="flex-1 px-3 py-4 space-y-1 overflow-y-auto">
        <div className="px-3 pb-2 text-[10px] font-semibold uppercase tracking-widest text-neutral-500">
          Navigation
        </div>
        {navLinks.map((link) => {
          const Icon = link.icon;
          const isActive =
            pathname === link.href ||
            (link.href !== "/dashboard" && pathname.startsWith(link.href));

          return (
            <Link
              key={link.href}
              href={link.href}
              onClick={() => setMobileOpen(false)}
              className={`flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-sm font-medium transition-all ${
                isActive
                  ? "bg-gradient-to-r from-amber-500/20 to-amber-600/10 border border-amber-500/40 text-white font-semibold shadow-[inset_0_1px_1px_rgba(255,255,255,0.15),0_0_15px_rgba(245,158,11,0.14)]"
                  : "text-neutral-400 hover:text-white hover:bg-white/[0.05] hover:border-white/[0.08] border border-transparent"
              }`}
            >
              <Icon
                className={`h-4.5 w-4.5 shrink-0 ${
                  isActive ? "text-amber-400 stroke-[2.2]" : "text-neutral-400 group-hover:text-neutral-200"
                }`}
              />
              <span className="truncate">{link.label}</span>
            </Link>
          );
        })}
      </nav>

      {/* Bottom User Profile Section */}
      <div className="p-3.5 border-t border-white/[0.07] bg-[#080a0e]/90 backdrop-blur-xl">
        {user ? (
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-2.5 min-w-0">
              <div className="h-9 w-9 rounded-full bg-gradient-to-br from-amber-400 via-amber-500 to-amber-600 text-black font-extrabold text-xs flex items-center justify-center shrink-0 shadow-[0_0_12px_rgba(245,158,11,0.25)]">
                {user.full_name
                  ?.split(" ")
                  .map((n) => n[0])
                  .join("")
                  .slice(0, 2)
                  .toUpperCase() || "MP"}
              </div>
              <div className="flex flex-col min-w-0">
                <span className="text-xs font-semibold text-white truncate">
                  {user.full_name || "Manthan Pathania"}
                </span>
                <span className="text-[11px] text-neutral-400 capitalize truncate">
                  {user.role === "ADMIN" ? "Admin" : "Candidate"}
                </span>
              </div>
            </div>
            <button
              onClick={() => logout()}
              className="p-2 rounded-lg text-neutral-500 hover:text-amber-400 hover:bg-white/[0.05] transition-colors"
              title="Sign Out"
              aria-label="Sign out"
            >
              <LogOut className="h-4 w-4" />
            </button>
          </div>
        ) : (
          <div className="space-y-2">
            <Link
              href="/login"
              onClick={() => setMobileOpen(false)}
              className="block w-full text-center py-2 px-3 rounded-xl text-xs font-semibold text-white border border-white/[0.1] hover:border-amber-500/40 hover:bg-amber-500/10 transition-all"
            >
              Sign In
            </Link>
            <Link
              href="/register"
              onClick={() => setMobileOpen(false)}
              className="block w-full text-center py-2 px-3 rounded-xl text-xs font-semibold bg-gradient-to-r from-amber-500 to-amber-600 text-black shadow-[0_0_15px_rgba(245,158,11,0.30)] hover:shadow-[0_0_25px_rgba(245,158,11,0.45)] transition-all"
            >
              Create Account
            </Link>
          </div>
        )}
      </div>
    </div>
  );

  return (
    <>
      {/* Mobile Top Header with Hamburger */}
      <div className="md:hidden sticky top-0 z-40 h-16 w-full bg-[#080a0e]/85 backdrop-blur-2xl border-b border-white/[0.07] px-4 flex items-center justify-between">
        <Link href={user ? "/dashboard" : "/"} className="flex items-center gap-2.5">
          <div className="h-8 w-8 rounded-lg bg-[#121520] border border-amber-500/40 flex items-center justify-center">
            <Code2 className="h-4 w-4 text-amber-400 stroke-[2.4]" />
          </div>
          <span className="font-extrabold text-sm tracking-wide text-white">
            Adaptive<span className="text-amber-400">DSA</span>
          </span>
        </Link>
        <button
          onClick={() => setMobileOpen(true)}
          className="p-2 rounded-lg text-neutral-300 hover:text-white hover:bg-white/[0.05] border border-white/[0.08] transition-colors"
          aria-label="Open navigation"
        >
          <Menu className="h-5 w-5" />
        </button>
      </div>

      {/* Mobile Drawer Backdrop */}
      {mobileOpen && (
        <div
          className="md:hidden fixed inset-0 z-50 bg-black/80 backdrop-blur-md"
          onClick={() => setMobileOpen(false)}
        >
          <div
            className="fixed inset-y-0 left-0 w-72 max-w-[85vw] bg-[#080a0e] border-r border-white/[0.08] shadow-2xl"
            onClick={(e) => e.stopPropagation()}
          >
            {renderNavContent()}
          </div>
        </div>
      )}

      {/* Desktop Fixed Left Sidebar */}
      <aside className="hidden md:flex fixed inset-y-0 left-0 w-64 z-30">
        {renderNavContent()}
      </aside>
    </>
  );
}
