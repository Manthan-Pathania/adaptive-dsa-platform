"use client";

import React, { useEffect, useState, useCallback, useMemo } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { api } from "@/lib/api";
import { PatternProgressItem, Roadmap, UserProfile } from "@/types";
import {
  Sparkles,
  Layers,
  ArrowUpRight,
  Search,
  RefreshCw,
  Zap,
  Calendar,
  ShieldCheck,
  BookOpen,
  Brain,
} from "lucide-react";

export default function DashboardPage() {
  const router = useRouter();
  const { user, credits, loading: authLoading } = useAuth();

  const [patternsProgress, setPatternsProgress] = useState<PatternProgressItem[]>([]);
  const [roadmaps, setRoadmaps] = useState<Roadmap[]>([]);
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [dueRevisionCount, setDueRevisionCount] = useState<number>(0);
  const [progressStats, setProgressStats] = useState<{
    total_solved: number;
    total_mastered: number;
    total_in_flight: number;
  }>({ total_solved: 0, total_mastered: 0, total_in_flight: 0 });

  const [searchFilter, setSearchFilter] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchDashboardData = useCallback(async () => {
    setError(null);
    try {
      const [patternsRes, progSummary, revisionsRes, roadmapsRes, userProf] = await Promise.all([
        api.getPatternsProgress().catch(() => ({ patterns: [] })),
        api.getProgressSummary().catch(() => ({
          stats: { total_solved: 0, total_mastered: 0, total_in_flight: 0 },
          recent_attempts: [],
        })),
        api.listDueRevisions(1, 0).catch(() => ({ revisions: [], total: 0 })),
        api.listRoadmaps().catch(() => ({ roadmaps: [] })),
        api.getProfile().catch(() => null),
      ]);

      setPatternsProgress(patternsRes.patterns || []);
      setProgressStats(progSummary.stats || { total_solved: 0, total_mastered: 0, total_in_flight: 0 });
      setDueRevisionCount(revisionsRes.total || 0);
      setRoadmaps(roadmapsRes.roadmaps || []);
      setProfile(userProf);
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message || "Failed to load dashboard metrics.");
    } finally {
      setIsLoading(false);
      setIsRefreshing(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login");
      return;
    }
    if (user) {
      fetchDashboardData();
    }
  }, [user, authLoading, router, fetchDashboardData]);

  const handleRefresh = () => {
    setIsRefreshing(true);
    fetchDashboardData();
  };

  // Active roadmap lookup
  const activeRoadmap = useMemo(() => {
    if (!profile?.selected_roadmap_id) {
      return roadmaps.find((r) => !r.is_system_template) || roadmaps[0] || null;
    }
    return roadmaps.find((r) => r.id === profile.selected_roadmap_id) || roadmaps[0] || null;
  }, [profile, roadmaps]);

  const masterRoadmap = useMemo(() => {
    return roadmaps.find((r) => r.is_system_template) || null;
  }, [roadmaps]);

  // Aggregate master calculations based on canonical Master Question Bank (539 problems)
  const totalMasterQuestions = useMemo(() => {
    if (masterRoadmap?.total_questions && masterRoadmap.total_questions > 0) {
      return masterRoadmap.total_questions;
    }
    return 539;
  }, [masterRoadmap]);

  const totalCompletedQuestions = useMemo(() => {
    return progressStats.total_solved + progressStats.total_mastered;
  }, [progressStats]);

  const masterCompletionPercent = useMemo(() => {
    if (totalMasterQuestions === 0) return 0;
    return Math.min(100, Math.round((totalCompletedQuestions / totalMasterQuestions) * 100));
  }, [totalCompletedQuestions, totalMasterQuestions]);

  // Filtered patterns for matrix
  const filteredPatterns = useMemo(() => {
    if (!searchFilter.trim()) return patternsProgress;
    const q = searchFilter.toLowerCase();
    return patternsProgress.filter(
      (p) => p.pattern_name.toLowerCase().includes(q) || p.topic_name.toLowerCase().includes(q)
    );
  }, [patternsProgress, searchFilter]);

  if (authLoading || (isLoading && patternsProgress.length === 0)) {
    return (
      <div className="p-6 md:p-10 max-w-7xl mx-auto w-full space-y-8 animate-pulse min-h-screen">
        <div className="h-10 bg-white/[0.04] rounded-xl w-1/3 border border-white/[0.06]" />
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-28 bg-[#10131d]/60 rounded-2xl border border-white/[0.06]" />
          ))}
        </div>
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div className="h-44 bg-[#10131d]/60 rounded-2xl border border-white/[0.06]" />
          <div className="h-44 bg-[#10131d]/60 rounded-2xl border border-white/[0.06]" />
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {[1, 2, 3, 4, 5, 6].map((i) => (
            <div key={i} className="h-28 bg-[#10131d]/60 rounded-xl border border-white/[0.06]" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto w-full space-y-8 text-white min-h-screen">
      {/* Executive Welcome & Top Bar */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 mb-2">
            <span className="w-1 h-3.5 bg-amber-400 rounded-sm inline-block shadow-[0_0_8px_rgba(245,158,11,0.5)]" />
            <span className="text-[11px] font-semibold text-neutral-400 tracking-widest uppercase">
              CANDIDATE INTELLIGENCE
            </span>
            {profile?.target_companies && profile.target_companies.length > 0 && (
              <span className="text-xs font-mono text-amber-400/90 ml-2">
                Targeting: {profile.target_companies.slice(0, 3).join(", ")}
              </span>
            )}
          </div>
          <h1 className="text-2xl md:text-3xl font-extrabold tracking-tight text-white">
            Candidate{" "}
            <span className="text-transparent bg-clip-text bg-gradient-to-r from-amber-300 via-amber-400 to-amber-500">
              Executive Dashboard
            </span>
          </h1>
          <p className="text-xs md:text-sm text-neutral-400 mt-1">
            Real-time pattern mastery, spaced repetition schedule, and FAANG preparation velocity.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={handleRefresh}
            disabled={isRefreshing}
            className="flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-medium text-neutral-300 border border-white/[0.1] bg-[#10131d]/60 backdrop-blur-xl hover:border-amber-400/40 hover:text-white transition-all shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)]"
            title="Refresh Metrics"
          >
            <RefreshCw className={`h-3.5 w-3.5 text-neutral-400 ${isRefreshing ? "animate-spin text-amber-400" : ""}`} />
            Refresh
          </button>
          <Link
            href="/roadmaps"
            className="flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold bg-gradient-to-r from-amber-500 to-amber-600 text-black shadow-[0_0_20px_rgba(245,158,11,0.28)] hover:shadow-[0_0_30px_rgba(245,158,11,0.45)] transition-all"
          >
            <Sparkles className="h-4 w-4 text-black stroke-[2.4]" />
            AI Roadmaps
          </Link>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-xl border border-amber-500/30 bg-amber-500/10 text-white flex items-center justify-between text-xs backdrop-blur-xl shadow-lg">
          <span>{error}</span>
          <button onClick={handleRefresh} className="underline text-amber-400 font-semibold">
            Retry
          </button>
        </div>
      )}

      {/* Row 1: 4 Core Executive Metric Glass Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {/* Metric 1: Total Completed (Soft Amber Highlight Glow) */}
        <div className="p-5 rounded-2xl bg-gradient-to-b from-[#181c2b]/75 to-[#0e111a]/75 backdrop-blur-2xl border border-amber-500/40 shadow-[inset_0_1px_1px_rgba(255,255,255,0.18),0_0_30px_rgba(245,158,11,0.10)] relative flex flex-col justify-between group">
          <div className="flex items-center justify-between">
            <div className="p-2 rounded-xl bg-amber-500/15 border border-amber-500/30 text-amber-400 shadow-sm">
              <Layers className="h-4 w-4 stroke-[2.2]" />
            </div>
            <span className="text-[11px] font-semibold uppercase tracking-wider text-neutral-400">
              TOTAL SOLVED
            </span>
          </div>
          <div className="mt-4 flex items-baseline gap-1.5">
            <span className="text-3xl font-extrabold text-white">{totalCompletedQuestions}</span>
            <span className="text-sm font-normal text-neutral-400">/ {totalMasterQuestions}</span>
          </div>
          <div className="mt-3 text-xs text-neutral-400">
            {masterCompletionPercent}% master bank solved
          </div>
        </div>

        {/* Metric 2: Needs Revision */}
        <Link
          href="/revisions"
          className="p-5 rounded-2xl bg-[#0e111a]/60 backdrop-blur-2xl border border-white/[0.09] hover:border-amber-400/40 hover:bg-[#131624]/70 shadow-[inset_0_1px_1px_rgba(255,255,255,0.10),0_12px_32px_rgba(0,0,0,0.45)] transition-all relative flex flex-col justify-between group block"
        >
          <div className="flex items-center justify-between">
            <div className="p-2 rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-400">
              <Calendar className="h-4 w-4 stroke-[2.2]" />
            </div>
            <span className="text-[11px] font-semibold uppercase tracking-wider text-neutral-400">
              DUE FOR REVISION
            </span>
          </div>
          <div className="mt-4 flex items-baseline gap-1.5">
            <span className="text-3xl font-extrabold text-white">{dueRevisionCount}</span>
            <span className="text-sm font-normal text-neutral-400">problems</span>
          </div>
          <div className="mt-3 text-xs text-neutral-400 group-hover:text-amber-400 transition-colors flex items-center gap-1">
            <span>SM-2 Memory Decay Active</span>
            <ArrowUpRight className="h-3.5 w-3.5" />
          </div>
        </Link>

        {/* Metric 3: Fully Confident / Mastered */}
        <div className="p-5 rounded-2xl bg-[#0e111a]/60 backdrop-blur-2xl border border-white/[0.09] hover:border-amber-400/40 hover:bg-[#131624]/70 shadow-[inset_0_1px_1px_rgba(255,255,255,0.10),0_12px_32px_rgba(0,0,0,0.45)] transition-all relative flex flex-col justify-between group">
          <div className="flex items-center justify-between">
            <div className="p-2 rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-400">
              <ShieldCheck className="h-4 w-4 stroke-[2.2]" />
            </div>
            <span className="text-[11px] font-semibold uppercase tracking-wider text-neutral-400">
              FULLY CONFIDENT
            </span>
          </div>
          <div className="mt-4 flex items-baseline gap-1.5">
            <span className="text-3xl font-extrabold text-white">{progressStats.total_mastered}</span>
            <span className="text-sm font-normal text-neutral-400">mastered</span>
          </div>
          <div className="mt-3 text-xs text-neutral-400">
            Solved independently 2+ times
          </div>
        </div>

        {/* Metric 4: AI Credits */}
        <Link
          href="/credits"
          className="p-5 rounded-2xl bg-[#0e111a]/60 backdrop-blur-2xl border border-white/[0.09] hover:border-amber-400/40 hover:bg-[#131624]/70 shadow-[inset_0_1px_1px_rgba(255,255,255,0.10),0_12px_32px_rgba(0,0,0,0.45)] transition-all relative flex flex-col justify-between group block"
        >
          <div className="flex items-center justify-between">
            <div className="p-2 rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-400">
              <Zap className="h-4 w-4 stroke-[2.2]" />
            </div>
            <span className="text-[11px] font-semibold uppercase tracking-wider text-neutral-400">
              AI TUTORING WALLET
            </span>
          </div>
          <div className="mt-4 flex items-baseline gap-1.5">
            <span className="text-3xl font-extrabold text-white font-mono">{credits}</span>
            <span className="text-sm font-normal text-neutral-400">credits</span>
          </div>
          <div className="mt-3 text-xs text-neutral-400 group-hover:text-amber-400 transition-colors flex items-center gap-1">
            <span>Manage AI Wallet</span>
            <ArrowUpRight className="h-3.5 w-3.5" />
          </div>
        </Link>
      </div>

      {/* Row 2: Visual Progress Velocity (Master Question Bank & Active Roadmap) */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Master Question Bank Card */}
        <div className="p-6 rounded-2xl bg-[#0e111a]/70 backdrop-blur-2xl border border-amber-500/30 shadow-[inset_0_1px_1px_rgba(255,255,255,0.15),0_16px_40px_rgba(0,0,0,0.5),0_0_25px_rgba(245,158,11,0.06)] space-y-5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-2.5 rounded-xl bg-amber-500/15 border border-amber-500/30 text-amber-400 shadow-sm">
                <Layers className="h-5 w-5 stroke-[2.2]" />
              </div>
              <div>
                <h2 className="text-base font-bold text-white">Master Question Bank</h2>
                <p className="text-xs text-neutral-400 font-mono">{totalMasterQuestions} Canonical FAANG Problems</p>
              </div>
            </div>
            <div className="px-3 py-1 rounded-lg bg-white/[0.05] border border-white/[0.1] text-sm font-bold text-white font-mono shadow-[inset_0_1px_1px_rgba(255,255,255,0.12)]">
              {masterCompletionPercent}%
            </div>
          </div>

          {/* Visual Sleek Amber Meter */}
          <div className="space-y-2">
            <div className="h-1.5 w-full rounded-full bg-neutral-900 border border-white/[0.06] overflow-hidden">
              <div
                className="h-full rounded-full bg-gradient-to-r from-amber-500 via-amber-400 to-amber-300 shadow-[0_0_12px_rgba(245,158,11,0.55)] transition-all duration-700 ease-out"
                style={{ width: `${Math.max(2, masterCompletionPercent)}%` }}
              />
            </div>
            <div className="flex justify-between text-xs font-medium">
              <span className="text-neutral-300">{totalCompletedQuestions} Solved</span>
              <span className="text-neutral-400">{Math.max(0, totalMasterQuestions - totalCompletedQuestions)} Remaining</span>
            </div>
          </div>

          <div className="pt-3 flex items-center justify-between border-t border-white/[0.06]">
            <span className="text-xs text-neutral-500">Ordered by Excel Sheet Progression</span>
            <Link
              href="/questions"
              className="text-xs font-semibold text-amber-400 hover:text-amber-300 flex items-center gap-1 transition-colors"
            >
              Open Master Bank <ArrowUpRight className="h-3.5 w-3.5" />
            </Link>
          </div>
        </div>

        {/* Active AI Roadmap Card */}
        <div className="p-6 rounded-2xl bg-[#0e111a]/70 backdrop-blur-2xl border border-white/[0.09] hover:border-amber-400/30 shadow-[inset_0_1px_1px_rgba(255,255,255,0.12),0_16px_40px_rgba(0,0,0,0.5)] transition-all space-y-5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div className="p-2.5 rounded-xl bg-amber-500/15 border border-amber-500/30 text-amber-400 shadow-sm">
                <BookOpen className="h-5 w-5 stroke-[2.2]" />
              </div>
              <div className="min-w-0">
                <h2 className="text-base font-bold text-white truncate">
                  {activeRoadmap ? activeRoadmap.title : "Master DSA Curriculum (29 Core Patterns)"}
                </h2>
                <p className="text-xs text-neutral-400 font-mono truncate">
                  {activeRoadmap?.target_days
                    ? `${activeRoadmap.target_days}-Day Accelerated Timeline`
                    : "90-Day Accelerated Timeline"}
                </p>
              </div>
            </div>
            <span className="px-2.5 py-0.5 rounded-full bg-amber-500/15 border border-amber-500/30 text-[10px] font-bold text-amber-400 tracking-wider uppercase shrink-0">
              ACTIVE
            </span>
          </div>

          <div className="space-y-2">
            <div className="h-1.5 w-full rounded-full bg-neutral-900 border border-white/[0.06] overflow-hidden">
              <div
                className="h-full rounded-full bg-gradient-to-r from-amber-500 via-amber-400 to-amber-300 shadow-[0_0_12px_rgba(245,158,11,0.55)] transition-all duration-700 ease-out"
                style={{ width: `${Math.max(2, masterCompletionPercent)}%` }}
              />
            </div>
            <div className="flex justify-between text-xs font-medium">
              <span className="text-neutral-300">
                {activeRoadmap?.target_days ? `${activeRoadmap.target_days}-Day Focus` : "Active Preparation Goal"}
              </span>
              <span className="text-neutral-400">
                {activeRoadmap?.total_questions ? `${activeRoadmap.total_questions} Questions` : "Synced Track"}
              </span>
            </div>
          </div>

          <div className="pt-3 flex items-center justify-between border-t border-white/[0.06]">
            <span className="text-xs text-neutral-500">Shared canonical progress</span>
            <Link
              href={activeRoadmap ? `/roadmaps/${activeRoadmap.slug}` : "/roadmaps"}
              className="text-xs font-semibold text-amber-400 hover:text-amber-300 flex items-center gap-1 transition-colors"
            >
              View Roadmap Details <ArrowUpRight className="h-3.5 w-3.5" />
            </Link>
          </div>
        </div>
      </div>

      {/* Row 3: Pattern Mastery Matrix (29 Algorithmic Patterns) */}
      <div className="space-y-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-xl bg-amber-500/15 border border-amber-500/25 text-amber-400 shadow-sm">
              <Brain className="h-5 w-5" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-white tracking-tight">
                Pattern Mastery Matrix
              </h2>
              <p className="text-xs text-neutral-400 mt-0.5">
                Comprehensive completion status across all 29 algorithmic patterns.
              </p>
            </div>
          </div>

          {/* Quick Search with Glass Input */}
          <div className="relative w-full sm:w-64">
            <Search className="h-4 w-4 absolute left-3 top-1/2 -translate-y-1/2 text-neutral-500" />
            <input
              type="text"
              placeholder="Filter patterns..."
              value={searchFilter}
              onChange={(e) => setSearchFilter(e.target.value)}
              className="w-full pl-9 pr-4 py-2 rounded-xl bg-[#10131d]/75 border border-white/[0.1] focus:border-amber-400/50 text-xs text-white placeholder-neutral-500 backdrop-blur-xl focus:outline-none transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
            />
          </div>
        </div>

        {/* 3-Column Grid of Frosted Glass Pattern Cards */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3.5">
          {filteredPatterns.map((pat) => {
            const pct = pat.percent ? Math.round(pat.percent) : 0;
            return (
              <Link
                key={pat.pattern_id}
                href={`/questions?pattern_id=${encodeURIComponent(pat.pattern_id)}`}
                className="p-4 rounded-xl bg-[#0e111a]/65 hover:bg-[#141825]/80 backdrop-blur-2xl border border-white/[0.08] hover:border-amber-400/40 shadow-[inset_0_1px_1px_rgba(255,255,255,0.10),0_8px_24px_rgba(0,0,0,0.4)] hover:shadow-[inset_0_1px_1px_rgba(255,255,255,0.18),0_12px_32px_rgba(0,0,0,0.5),0_0_20px_rgba(245,158,11,0.08)] transition-all duration-200 group flex flex-col justify-between"
              >
                <div>
                  <div className="flex items-start justify-between gap-2">
                    <span className="text-[10px] font-bold text-amber-400/90 uppercase tracking-wider">
                      {pat.pattern_name.toUpperCase()}
                    </span>
                    <span className="text-xs font-bold text-white font-mono">
                      {pct}%
                    </span>
                  </div>
                  <h3 className="font-semibold text-sm text-white mt-1 group-hover:text-amber-300 transition-colors">
                    {pat.pattern_name}
                  </h3>
                </div>

                <div className="mt-3.5 space-y-1.5">
                  <div className="h-1.5 w-full rounded-full bg-neutral-900 border border-white/[0.04] overflow-hidden">
                    <div
                      className="h-full rounded-full bg-gradient-to-r from-amber-500 to-amber-400 transition-all duration-500"
                      style={{ width: `${Math.max(pct > 0 ? 3 : 0, pct)}%` }}
                    />
                  </div>
                  <div className="flex justify-between items-center text-xs font-mono text-neutral-400">
                    <span>
                      {pat.solved_count} / {pat.total_questions} Solved
                    </span>
                    <span className="text-xs font-medium text-amber-400 group-hover:translate-x-0.5 transition-transform flex items-center gap-0.5">
                      Filter →
                    </span>
                  </div>
                </div>
              </Link>
            );
          })}
        </div>
      </div>
    </div>
  );
}
