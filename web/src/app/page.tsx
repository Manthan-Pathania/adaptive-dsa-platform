"use client";

import Link from "next/link";
import { useAuth } from "@/lib/auth-context";
import {
  Brain,
  Sparkles,
  Target,
  Clock,
  ArrowRight,
  CheckCircle2,
  Zap,
  TrendingUp,
  ShieldCheck,
  Code2,
} from "lucide-react";

export default function HomePage() {
  const { user } = useAuth();

  return (
    <div className="flex flex-col flex-1">
      {/* Hero Section */}
      <section className="relative overflow-hidden py-20 lg:py-28 border-b border-slate-850">
        <div className="absolute inset-0 bg-[radial-gradient(ellipse_80%_80%_at_50%_-20%,rgba(16,185,129,0.15),rgba(255,255,255,0))] -z-10" />
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex flex-col items-center text-center">
          <div className="inline-flex items-center gap-2 px-3.5 py-1.5 rounded-full bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs font-semibold mb-8 animate-fade-in">
            <Sparkles className="h-3.5 w-3.5" />
            Adaptive Spaced Repetition & Gemini-Powered Tutoring
          </div>

          <h1 className="text-4xl sm:text-6xl lg:text-7xl font-extrabold tracking-tight max-w-4xl bg-gradient-to-b from-white via-slate-100 to-slate-400 bg-clip-text text-transparent">
            Master Technical Interviews <br className="hidden sm:inline" />
            <span className="bg-gradient-to-r from-emerald-400 to-teal-400 bg-clip-text text-transparent">
              Without the Burnout.
            </span>
          </h1>

          <p className="mt-6 text-lg sm:text-xl text-slate-400 max-w-2xl leading-relaxed">
            Stop grinding random LeetCode lists. Our deterministic recommendation engine prioritizes
            your weakest algorithmic patterns, enforces SM-2 spaced repetition, and provides
            progressive AI hints that guide without spoiling the solution.
          </p>

          <div className="mt-10 flex flex-col sm:flex-row items-center gap-4 w-full sm:w-auto">
            {user ? (
              <Link
                href="/dashboard"
                className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-8 py-3.5 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-base transition-all shadow-lg shadow-emerald-500/25 group"
              >
                Go to Daily Dashboard
                <ArrowRight className="h-4 w-4 group-hover:translate-x-1 transition-transform" />
              </Link>
            ) : (
              <>
                <Link
                  href="/register"
                  className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-8 py-3.5 rounded-xl bg-emerald-500 hover:bg-emerald-400 text-slate-950 font-bold text-base transition-all shadow-lg shadow-emerald-500/25 group"
                >
                  Start Preparation Free
                  <ArrowRight className="h-4 w-4 group-hover:translate-x-1 transition-transform" />
                </Link>
                <Link
                  href="/login"
                  className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-7 py-3.5 rounded-xl bg-slate-900 hover:bg-slate-800 text-slate-200 border border-slate-750 font-semibold text-base transition-all"
                >
                  Sign In
                </Link>
              </>
            )}
            <Link
              href="/questions"
              className="w-full sm:w-auto inline-flex items-center justify-center gap-2 px-7 py-3.5 rounded-xl bg-slate-900/60 hover:bg-slate-800 text-slate-300 border border-slate-800 font-semibold text-base transition-all"
            >
              <Code2 className="h-4 w-4 text-emerald-400" />
              Explore Question Catalog
            </Link>
          </div>

          {/* Quick Onboarding Value Badges */}
          <div className="mt-12 flex flex-wrap justify-center items-center gap-6 sm:gap-10 text-xs text-slate-400">
            <div className="flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4 text-emerald-400" />
              <span>50 Welcome Credits Included</span>
            </div>
            <div className="flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4 text-emerald-400" />
              <span>SuperMemo SM-2 Retention Engine</span>
            </div>
            <div className="flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4 text-emerald-400" />
              <span>4-Tier Socratic AI Hint Ladder</span>
            </div>
          </div>
        </div>
      </section>

      {/* Feature Grid */}
      <section className="py-20 bg-slate-950/60">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="text-center max-w-2xl mx-auto mb-16">
            <h2 className="text-xs font-mono uppercase tracking-widest text-emerald-400 mb-3">
              Engine Architecture
            </h2>
            <h3 className="text-3xl sm:text-4xl font-bold tracking-tight text-white">
              Built for Rapid, Retained Mastery
            </h3>
            <p className="mt-4 text-slate-400">
              Unlike static curated lists, our engine dynamically tracks your memory decay curve and
              mistake taxonomy to guarantee interview readiness.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            {/* Card 1 */}
            <div className="p-8 rounded-2xl bg-slate-900/50 border border-slate-800/80 hover:border-emerald-500/40 transition-all group">
              <div className="h-12 w-12 rounded-xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center mb-6 group-hover:scale-105 transition-transform">
                <Target className="h-6 w-6 text-emerald-400" />
              </div>
              <h4 className="text-xl font-bold text-white mb-2">Deterministic Daily Plan</h4>
              <p className="text-slate-400 text-sm leading-relaxed mb-4">
                Prioritizes due SM-2 spaced reviews first, then targets your historically weakest
                patterns before suggesting roadmap next-steps and curated explorations.
              </p>
              <div className="flex items-center gap-2 text-xs font-mono text-emerald-400">
                <span>Due Reviews &gt; Weak Patterns &gt; Roadmap</span>
              </div>
            </div>

            {/* Card 2 */}
            <div className="p-8 rounded-2xl bg-slate-900/50 border border-slate-800/80 hover:border-emerald-500/40 transition-all group">
              <div className="h-12 w-12 rounded-xl bg-teal-500/10 border border-teal-500/20 flex items-center justify-center mb-6 group-hover:scale-105 transition-transform">
                <Brain className="h-6 w-6 text-teal-400" />
              </div>
              <h4 className="text-xl font-bold text-white mb-2">Progressive AI Hint Ladder</h4>
              <p className="text-slate-400 text-sm leading-relaxed mb-4">
                Never jump straight to the editorial. Request a Level 1 conceptual nudge (2 credits),
                algorithmic direction (2 credits), key data structure choice (4 credits), or
                pseudocode outline (4 credits).
              </p>
              <div className="flex items-center gap-2 text-xs font-mono text-teal-400">
                <span>Socratic Guidance &bull; Zero Leaks</span>
              </div>
            </div>

            {/* Card 3 */}
            <div className="p-8 rounded-2xl bg-slate-900/50 border border-slate-800/80 hover:border-emerald-500/40 transition-all group">
              <div className="h-12 w-12 rounded-xl bg-indigo-500/10 border border-indigo-500/20 flex items-center justify-center mb-6 group-hover:scale-105 transition-transform">
                <Clock className="h-6 w-6 text-indigo-400" />
              </div>
              <h4 className="text-xl font-bold text-white mb-2">SM-2 Spaced Repetition</h4>
              <p className="text-slate-400 text-sm leading-relaxed mb-4">
                Every attempt recalculates the question stability, ease factor, and repetition interval.
                Hard questions reappear sooner; mastered patterns space out over weeks.
              </p>
              <div className="flex items-center gap-2 text-xs font-mono text-indigo-400">
                <span>Scientifically Proven Memory Retention</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Trust & Transparency */}
      <section className="py-16 border-t border-slate-850 bg-slate-900/20">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6 text-center sm:text-left">
            <div className="flex items-start gap-4">
              <div className="p-3 rounded-lg bg-emerald-500/10 text-emerald-400">
                <Zap className="h-6 w-6" />
              </div>
              <div>
                <h5 className="font-semibold text-white">Immutable Credit Ledger</h5>
                <p className="text-xs text-slate-400 mt-1">
                  Every credit deduction, grant, and refund is recorded in an append-only ledger with
                  atomic database row-level locking.
                </p>
              </div>
            </div>

            <div className="flex items-start gap-4">
              <div className="p-3 rounded-lg bg-teal-500/10 text-teal-400">
                <ShieldCheck className="h-6 w-6" />
              </div>
              <div>
                <h5 className="font-semibold text-white">Zero-Charge Guarantee</h5>
                <p className="text-xs text-slate-400 mt-1">
                  If the AI tutoring provider experiences a timeout, rate limit, or model error, your
                  credits are automatically protected and never deducted.
                </p>
              </div>
            </div>

            <div className="flex items-start gap-4">
              <div className="p-3 rounded-lg bg-indigo-500/10 text-indigo-400">
                <TrendingUp className="h-6 w-6" />
              </div>
              <div>
                <h5 className="font-semibold text-white">Mistake Tag Taxonomy</h5>
                <p className="text-xs text-slate-400 mt-1">
                  Categorize failures by edge case omissions, off-by-one errors, or data structure
                  misidentifications to eliminate blind spots.
                </p>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
