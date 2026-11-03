"use client";

import React, { useEffect, useState, useCallback, useMemo } from "react";
import Link from "next/link";
import { useSearchParams, useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { Question, Pattern, UserQuestionProgress, AttemptOutcome, PerceivedDifficulty } from "@/types";
import { QuickMarkModal } from "@/components/QuickMarkModal";
import {
  Search,
  ExternalLink,
  Filter,
  Check,
  Layers,
  ChevronLeft,
  ChevronRight,
  BookOpen,
} from "lucide-react";

export default function MasterQuestionBankPage() {
  const { user } = useAuth();
  const router = useRouter();
  const searchParams = useSearchParams();
  const initialPatternId = searchParams.get("pattern_id") || "";

  const [questions, setQuestions] = useState<Question[]>([]);
  const [patterns, setPatterns] = useState<Pattern[]>([]);
  const [total, setTotal] = useState(0);

  const [search, setSearch] = useState("");
  const [selectedPatternId, setSelectedPatternId] = useState(initialPatternId);
  const [difficultyFilter, setDifficultyFilter] = useState("");
  const [page, setPage] = useState(1);
  const perPage = 50;

  const [progressMap, setProgressMap] = useState<Record<string, UserQuestionProgress>>({});
  const [activeModalQuestion, setActiveModalQuestion] = useState<Question | null>(null);
  const [isSubmittingQuickMark, setIsSubmittingQuickMark] = useState(false);

  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch patterns list
  useEffect(() => {
    api
      .listPatterns()
      .then((data) => setPatterns(data.patterns || []))
      .catch(() => {});
  }, []);

  // Fetch user progress map if authenticated
  const fetchUserProgress = useCallback(async () => {
    if (!user) return;
    try {
      const res = await api.listAllUserProgress();
      const map: Record<string, UserQuestionProgress> = {};
      (res.progress || []).forEach((p) => {
        map[p.question_id] = p;
      });
      setProgressMap(map);
    } catch {
      // Non-blocking
    }
  }, [user]);

  useEffect(() => {
    fetchUserProgress();
  }, [fetchUserProgress]);

  // Fetch questions
  const fetchQuestions = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const data = await api.listQuestions({
        search: search.trim() || undefined,
        pattern_id: selectedPatternId || undefined,
        difficulty: difficultyFilter || undefined,
        limit: perPage,
        offset: (page - 1) * perPage,
      });
      setQuestions(data.questions || []);
      setTotal(data.total || 0);
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message || "Failed to load master questions.");
    } finally {
      setIsLoading(false);
    }
  }, [search, selectedPatternId, difficultyFilter, page]);

  useEffect(() => {
    fetchQuestions();
  }, [fetchQuestions]);

  // Handle Checkbox Click
  const handleCheckboxClick = async (q: Question) => {
    if (!user) {
      router.push("/login");
      return;
    }

    const currentProg = progressMap[q.id];
    const isSolved = currentProg?.status === "SOLVED" || currentProg?.status === "MASTERED";

    if (isSolved) {
      // Unmark directly
      try {
        await api.unmarkProgress(q.id);
        setProgressMap((prev) => {
          const updated = { ...prev };
          delete updated[q.id];
          return updated;
        });
      } catch (err) {
        console.error("Failed to unmark question:", err);
      }
    } else {
      // Open QuickMarkModal
      setActiveModalQuestion(q);
    }
  };

  const handleModalSubmit = async (data: {
    outcome: AttemptOutcome;
    difficulty_experienced: PerceivedDifficulty;
    needs_revision: boolean;
    confidence_first_try: boolean;
    notes: string;
  }) => {
    if (!activeModalQuestion) return;
    setIsSubmittingQuickMark(true);
    try {
      const res = await api.quickMarkProgress({
        question_id: activeModalQuestion.id,
        outcome: data.outcome,
        difficulty_experienced: data.difficulty_experienced,
        needs_revision: data.needs_revision,
        confidence_first_try: data.confidence_first_try,
        notes: data.notes,
      });

      if (res.progress) {
        setProgressMap((prev) => ({
          ...prev,
          [activeModalQuestion.id]: res.progress,
        }));
      }
      setActiveModalQuestion(null);
    } catch (err) {
      console.error("Failed to quick mark question:", err);
    } finally {
      setIsSubmittingQuickMark(false);
    }
  };

  const totalPages = Math.ceil(total / perPage) || 1;

  const totalSolvedCount = useMemo(() => {
    return Object.values(progressMap).filter(
      (p) => p.status === "SOLVED" || p.status === "MASTERED"
    ).length;
  }, [progressMap]);

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto w-full space-y-6 text-white min-h-screen">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 mb-2">
            <span className="w-1 h-3.5 bg-amber-400 rounded-sm inline-block shadow-[0_0_8px_rgba(245,158,11,0.5)]" />
            <span className="text-[11px] font-semibold text-neutral-400 tracking-widest uppercase">
              CANONICAL PROGRESS
            </span>
            <span className="text-xs font-mono text-amber-400/90 ml-2">
              Excel Sheet Progression (1 to 539)
            </span>
          </div>
          <h1 className="text-2xl md:text-3xl font-extrabold tracking-tight text-white flex items-center gap-3">
            <div className="p-2 rounded-xl bg-amber-500/15 border border-amber-500/30 text-amber-400 shadow-sm">
              <Layers className="h-6 w-6 stroke-[2.2]" />
            </div>
            Master Question Bank
          </h1>
          <p className="text-xs md:text-sm text-neutral-400 mt-1">
            The full authoritative FAANG curriculum ordered strictly by algorithmic progression. Solved status reflects universally across all roadmaps.
          </p>
        </div>

        {/* Global Solved Counter Glass Box */}
        <div className="flex items-center gap-4 bg-[#0e111a]/70 backdrop-blur-2xl border border-white/[0.09] px-5 py-3 rounded-2xl shrink-0 shadow-[inset_0_1px_1px_rgba(255,255,255,0.12),0_12px_28px_rgba(0,0,0,0.4)]">
          <div className="flex flex-col">
            <span className="text-[10px] font-mono uppercase tracking-wider text-neutral-400">
              Master Solved
            </span>
            <span className="text-base font-extrabold font-mono text-white">
              <span className="text-amber-400">{totalSolvedCount}</span> / {total || 539}
            </span>
          </div>
          <div className="h-8 w-px bg-white/[0.08]" />
          <div className="flex flex-col">
            <span className="text-[10px] font-mono uppercase tracking-wider text-neutral-400">
              Completion
            </span>
            <span className="text-base font-extrabold font-mono text-amber-400">
              {total > 0 ? Math.round((totalSolvedCount / total) * 100) : 0}%
            </span>
          </div>
        </div>
      </div>

      {/* Filter Bar: Pattern Only + Search + Difficulty */}
      <div className="p-4 rounded-2xl bg-[#0e111a]/65 backdrop-blur-2xl border border-white/[0.09] shadow-[inset_0_1px_1px_rgba(255,255,255,0.10),0_8px_24px_rgba(0,0,0,0.35)] flex flex-col lg:flex-row gap-4 items-center justify-between">
        {/* Search */}
        <div className="relative w-full lg:w-80">
          <Search className="h-4 w-4 absolute left-3.5 top-1/2 -translate-y-1/2 text-neutral-500" />
          <input
            type="text"
            placeholder="Search problems by title..."
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(1);
            }}
            className="w-full pl-10 pr-3 py-2 rounded-xl bg-[#121520]/80 border border-white/[0.1] focus:border-amber-400/50 text-xs text-white placeholder-neutral-500 focus:outline-none transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
          />
        </div>

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-3 w-full lg:w-auto">
          {/* Pattern Only Filter */}
          <div className="flex items-center gap-2">
            <Filter className="h-3.5 w-3.5 text-amber-400" />
            <select
              value={selectedPatternId}
              onChange={(e) => {
                setSelectedPatternId(e.target.value);
                setPage(1);
              }}
              className="py-2 px-3 rounded-xl bg-[#121520]/80 border border-white/[0.1] focus:border-amber-400/50 text-xs text-white focus:outline-none max-w-[240px] transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
            >
              <option value="">All 29 Patterns</option>
              {patterns.map((p) => (
                <option key={p.id} value={p.id} className="bg-[#0e111a] text-white">
                  {p.name}
                </option>
              ))}
            </select>
          </div>

          {/* Difficulty Filter */}
          <select
            value={difficultyFilter}
            onChange={(e) => {
              setDifficultyFilter(e.target.value);
              setPage(1);
            }}
            className="py-2 px-3 rounded-xl bg-[#121520]/80 border border-white/[0.1] focus:border-amber-400/50 text-xs text-white focus:outline-none transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
          >
            <option value="">All Difficulties</option>
            <option value="EASY">Easy</option>
            <option value="MEDIUM">Medium</option>
            <option value="HARD">Hard</option>
          </select>

          {(search || selectedPatternId || difficultyFilter) && (
            <button
              onClick={() => {
                setSearch("");
                setSelectedPatternId("");
                setDifficultyFilter("");
                setPage(1);
              }}
              className="text-xs text-amber-400 hover:underline font-mono"
            >
              Clear filters
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-xl border border-amber-500/30 bg-amber-500/10 text-white text-xs backdrop-blur-xl">
          {error}
        </div>
      )}

      {/* Questions Table with Frosted Glass */}
      <div className="rounded-2xl border border-white/[0.09] bg-[#0e111a]/70 backdrop-blur-3xl overflow-hidden shadow-[inset_0_1px_1px_rgba(255,255,255,0.12),0_16px_40px_rgba(0,0,0,0.5)]">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs border-collapse">
            <thead>
              <tr className="border-b border-white/[0.08] bg-[#080a10]/85 text-neutral-400 font-mono uppercase text-[11px]">
                <th className="py-3.5 px-4 w-12 text-center">Status</th>
                <th className="py-3.5 px-3 w-16"># Order</th>
                <th className="py-3.5 px-4">Problem Title</th>
                <th className="py-3.5 px-4">Pattern & Subpattern</th>
                <th className="py-3.5 px-3 w-24 text-center">Difficulty</th>
                <th className="py-3.5 px-4 w-28 text-center">Workspace</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-white/[0.04]">
              {isLoading ? (
                Array.from({ length: 10 }).map((_, i) => (
                  <tr key={i} className="animate-pulse">
                    <td className="py-4 px-4 text-center">
                      <div className="h-4 w-4 bg-white/[0.05] rounded mx-auto" />
                    </td>
                    <td className="py-4 px-3">
                      <div className="h-4 w-8 bg-white/[0.05] rounded" />
                    </td>
                    <td className="py-4 px-4">
                      <div className="h-4 w-48 bg-white/[0.05] rounded" />
                    </td>
                    <td className="py-4 px-4">
                      <div className="h-4 w-32 bg-white/[0.05] rounded" />
                    </td>
                    <td className="py-4 px-3">
                      <div className="h-4 w-16 bg-white/[0.05] rounded mx-auto" />
                    </td>
                    <td className="py-4 px-4">
                      <div className="h-4 w-20 bg-white/[0.05] rounded mx-auto" />
                    </td>
                  </tr>
                ))
              ) : questions.length === 0 ? (
                <tr>
                  <td colSpan={6} className="py-12 text-center text-neutral-500 font-mono">
                    No questions found matching your filter criteria.
                  </td>
                </tr>
              ) : (
                questions.map((q) => {
                  const qId = q.id;
                  const prog = progressMap[qId];
                  const isSolved = prog?.status === "SOLVED" || prog?.status === "MASTERED";
                  const isMastered = prog?.status === "MASTERED";

                  const patternBadge = q.patterns && q.patterns.length > 0 ? q.patterns[0].name : "General";
                  const subpatternBadge = q.patterns && q.patterns.length > 1 ? q.patterns[1].name : null;

                  return (
                    <tr
                      key={qId}
                      className={`hover:bg-white/[0.035] transition-colors ${
                        isSolved ? "bg-amber-500/[0.04]" : ""
                      }`}
                    >
                      {/* Checkbox Quick-Mark */}
                      <td className="py-3.5 px-4 text-center">
                        <button
                          type="button"
                          onClick={() => handleCheckboxClick(q)}
                          className={`h-5 w-5 rounded-md border flex items-center justify-center transition-all ${
                            isSolved
                              ? "bg-gradient-to-r from-amber-500 to-amber-600 border-amber-500 text-black shadow-[0_0_12px_rgba(245,158,11,0.45)]"
                              : "border-white/[0.15] bg-[#121520] hover:border-amber-400/60"
                          }`}
                          title={isSolved ? "Marked as solved (Click to unmark)" : "Click to log completion"}
                        >
                          {isSolved && <Check className="h-3.5 w-3.5 stroke-[3]" />}
                        </button>
                      </td>

                      {/* Excel Order Index */}
                      <td className="py-3.5 px-3 font-mono text-neutral-400 text-xs">
                        #{q.excel_order || "-"}
                      </td>

                      {/* Problem Title & Practice Link */}
                      <td className="py-3.5 px-4 font-medium text-white">
                        <div className="flex items-center gap-2">
                          <Link
                            href={`/questions/${qId}`}
                            className="hover:text-amber-400 hover:underline transition-colors leading-tight"
                          >
                            {q.title}
                          </Link>
                          {isMastered && (
                            <span className="text-[10px] font-mono font-bold text-amber-400 border border-amber-500/40 px-1 rounded bg-amber-500/10">
                              ★ MASTERED
                            </span>
                          )}
                          {q.external_url && (
                            <a
                              href={q.external_url}
                              target="_blank"
                              rel="noreferrer"
                              className="text-neutral-500 hover:text-amber-400 transition-colors"
                              title="Open on LeetCode"
                            >
                              <ExternalLink className="h-3.5 w-3.5" />
                            </a>
                          )}
                        </div>
                      </td>

                      {/* Pattern & Subpattern Badges */}
                      <td className="py-3.5 px-4">
                        <div className="flex flex-wrap items-center gap-1.5">
                          <span className="px-2 py-0.5 rounded-md text-[10px] font-mono bg-amber-500/10 text-amber-400 border border-amber-500/25 font-semibold">
                            {patternBadge}
                          </span>
                          {subpatternBadge && (
                            <span className="px-2 py-0.5 rounded-md text-[10px] font-mono bg-white/[0.04] text-neutral-400 border border-white/[0.08]">
                              {subpatternBadge}
                            </span>
                          )}
                        </div>
                      </td>

                      {/* Difficulty */}
                      <td className="py-3.5 px-3 text-center">
                        <span
                          className={`px-2 py-0.5 rounded font-mono text-[10px] uppercase font-bold border ${
                            q.difficulty === "EASY"
                              ? "text-neutral-300 border-white/20 bg-white/[0.03]"
                              : q.difficulty === "MEDIUM"
                              ? "text-amber-400 border-amber-500/35 bg-amber-500/10"
                              : "text-amber-300 border-amber-400/60 bg-amber-500/20 font-extrabold"
                          }`}
                        >
                          {q.difficulty}
                        </span>
                      </td>

                      {/* Practice Workspace CTA */}
                      <td className="py-3.5 px-4 text-center">
                        <Link
                          href={`/questions/${qId}`}
                          className="inline-flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-bold text-black bg-gradient-to-r from-amber-500 to-amber-600 hover:shadow-[0_0_15px_rgba(245,158,11,0.35)] transition-all"
                        >
                          <BookOpen className="h-3 w-3 stroke-[2.5]" />
                          Solve
                        </Link>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination Bar */}
        <div className="p-4 border-t border-white/[0.08] bg-[#080a10]/80 flex items-center justify-between font-mono text-xs text-neutral-400">
          <div>
            Showing {(page - 1) * perPage + 1} - {Math.min(page * perPage, total)} of {total} questions
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setPage((p) => Math.max(1, p - 1))}
              disabled={page <= 1}
              className="p-1.5 rounded-lg border border-white/[0.1] bg-[#121520] hover:border-amber-400/50 disabled:opacity-30 disabled:pointer-events-none transition-colors"
            >
              <ChevronLeft className="h-4 w-4 text-amber-400" />
            </button>
            <span className="text-white font-bold px-2">
              Page {page} of {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
              disabled={page >= totalPages}
              className="p-1.5 rounded-lg border border-white/[0.1] bg-[#121520] hover:border-amber-400/50 disabled:opacity-30 disabled:pointer-events-none transition-colors"
            >
              <ChevronRight className="h-4 w-4 text-amber-400" />
            </button>
          </div>
        </div>
      </div>

      {/* Quick Mark Modal */}
      {activeModalQuestion && (
        <QuickMarkModal
          isOpen={true}
          questionTitle={activeModalQuestion.title}
          questionDifficulty={activeModalQuestion.difficulty}
          patternName={
            activeModalQuestion.patterns && activeModalQuestion.patterns.length > 0
              ? activeModalQuestion.patterns[0].name
              : undefined
          }
          onClose={() => setActiveModalQuestion(null)}
          onSubmit={handleModalSubmit}
          isSubmitting={isSubmittingQuickMark}
        />
      )}
    </div>
  );
}
