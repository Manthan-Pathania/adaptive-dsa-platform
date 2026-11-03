"use client";

import React, { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { api } from "@/lib/api";
import { RevisionSchedule } from "@/types";
import {
  CheckCircle2,
  AlertTriangle,
  ArrowRight,
  RefreshCw,
  Calendar,
  RotateCw,
} from "lucide-react";

export default function RevisionsPage() {
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();
  const [revisions, setRevisions] = useState<RevisionSchedule[]>([]);
  const [total, setTotal] = useState(0);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchRevisions = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await api.listDueRevisions(50, 0);
      setRevisions(data.revisions || []);
      setTotal(data.total || 0);
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message || "Failed to load due revisions.");
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login");
      return;
    }
    if (user) {
      fetchRevisions();
    }
  }, [user, authLoading, router, fetchRevisions]);

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto w-full space-y-8 text-white min-h-screen">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 mb-2">
            <span className="w-1.5 h-3.5 bg-amber-400 rounded-sm inline-block shadow-[0_0_8px_rgba(245,158,11,0.5)]" />
            <span className="text-[11px] font-semibold text-neutral-400 tracking-widest uppercase">
              SUPERMEMO SM-2
            </span>
            <span className="text-xs font-mono text-amber-400/90 ml-2">Memory Retention Engine</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-white tracking-tight flex items-center gap-3">
            <div className="p-2 rounded-xl bg-amber-500/15 text-amber-400 border border-amber-500/30 shadow-[0_0_15px_rgba(245,158,11,0.15)]">
              <RotateCw className="h-6 w-6 stroke-[2.2]" />
            </div>
            Due Spaced Revisions
          </h1>
          <p className="text-xs sm:text-sm text-neutral-400 mt-1">
            Questions flagged for retention consolidation to prevent algorithmic forgetting.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={fetchRevisions}
            disabled={isLoading}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-[#0e1017]/75 border border-white/[0.08] hover:border-amber-400/50 text-xs font-semibold text-neutral-300 hover:text-white transition-colors backdrop-blur-2xl shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)]"
          >
            <RefreshCw className={`h-3.5 w-3.5 text-neutral-400 ${isLoading ? "animate-spin text-amber-400" : ""}`} />
            Refresh
          </button>
          <div className="px-4 py-2 rounded-xl bg-[#0e1017]/75 border border-white/[0.08] text-xs font-mono text-neutral-300 backdrop-blur-2xl shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)]">
            Due Now: <strong className="text-amber-400">{total}</strong>
          </div>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-200 text-xs backdrop-blur-md">
          {error}
        </div>
      )}

      {/* Revisions List */}
      {isLoading ? (
        <div className="space-y-3 animate-pulse">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-24 bg-[#0e1017]/60 rounded-2xl border border-white/[0.06]" />
          ))}
        </div>
      ) : revisions.length > 0 ? (
        <div className="space-y-3">
          {revisions.map((rev) => {
            const nextDate = new Date(rev.next_review_at);
            const isOverdue = nextDate < new Date();
            return (
              <div
                key={rev.id}
                className="p-5 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] hover:border-amber-400/40 hover:bg-[#121520]/80 transition-all flex flex-col sm:flex-row sm:items-center justify-between gap-4 group shadow-[inset_0_1px_1px_rgba(255,255,255,0.08),0_8px_24px_rgba(0,0,0,0.35)]"
              >
                <div className="space-y-2.5 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="px-2 py-0.5 rounded text-[10px] font-bold border uppercase font-mono bg-amber-500/10 text-amber-400 border-amber-500/30">
                      {rev.question_difficulty}
                    </span>
                    {isOverdue ? (
                      <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded text-[10px] font-bold bg-amber-500/20 text-amber-300 border border-amber-400/50">
                        <AlertTriangle className="h-3 w-3" /> Overdue Review
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 px-2.5 py-0.5 rounded text-[10px] font-bold bg-white/[0.04] text-neutral-300 border border-white/[0.1]">
                        <Calendar className="h-3 w-3 text-amber-400" /> Due Today
                      </span>
                    )}
                  </div>

                  <h3 className="text-base font-bold text-white group-hover:text-amber-300 transition-colors">
                    {rev.question_title}
                  </h3>

                  <div className="flex flex-wrap items-center gap-4 text-xs font-mono text-neutral-400">
                    <span>
                      Interval: <strong className="text-white">{rev.interval_days}d</strong>
                    </span>
                    <span>
                      Repetitions: <strong className="text-white">{rev.repetition_count}</strong>
                    </span>
                    <span>
                      Ease Factor: <strong className="text-amber-400">{rev.ease_factor.toFixed(2)}</strong>
                    </span>
                    <span>
                      Target: <strong className="text-white">{nextDate.toLocaleDateString()}</strong>
                    </span>
                  </div>
                </div>

                <div className="shrink-0 self-end sm:self-center">
                  <Link
                    href={`/questions/${rev.question_id}`}
                    className="inline-flex items-center gap-2 px-4 py-2.5 rounded-xl bg-gradient-to-r from-amber-400 to-amber-500 text-black font-bold text-xs transition-all shadow-[0_0_15px_rgba(245,158,11,0.25)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)]"
                  >
                    <span>Practice Now</span>
                    <ArrowRight className="h-3.5 w-3.5" />
                  </Link>
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className="p-12 rounded-2xl bg-[#0e1017]/70 backdrop-blur-2xl border border-white/[0.08] text-center space-y-3 shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)]">
          <CheckCircle2 className="h-10 w-10 text-amber-400 mx-auto" />
          <h3 className="text-base font-bold text-white">All Caught Up!</h3>
          <p className="text-xs text-neutral-400 max-w-sm mx-auto">
            You have zero due spaced revisions today. The SM-2 algorithm schedules upcoming problems as memory decays.
          </p>
          <Link
            href="/questions"
            className="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-[#141722]/80 border border-white/[0.08] hover:border-amber-400/40 text-xs font-semibold text-amber-400 mt-2 transition-colors shadow-sm"
          >
            Open Master Question Bank
          </Link>
        </div>
      )}
    </div>
  );
}
