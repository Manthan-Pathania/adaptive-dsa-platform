"use client";

import React, { useEffect, useState, useMemo, use } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import {
  Roadmap,
  RoadmapItem,
  UserProfile,
  UserQuestionProgress,
  AttemptOutcome,
  PerceivedDifficulty,
} from "@/types";
import { QuickMarkModal } from "@/components/QuickMarkModal";
import {
  ArrowLeft,
  Check,
  BookOpen,
  Layers,
  ListOrdered,
  Sparkles,
  Search,
  Filter,
  ChevronDown,
  ChevronRight,
} from "lucide-react";

const STAGE_CONFIG: Record<number, { title: string; subtitle: string }> = {
  1: {
    title: "Stage 1: Core Patterns & Prerequisites",
    subtitle: "Foundational techniques in strict dependency order. Focus on intuition and derivation.",
  },
  2: {
    title: "Stage 2: Targeted Strengthening & Variations",
    subtitle: "Non-trivial pattern combinations, edge-case hardening, and medium-complexity variants.",
  },
  3: {
    title: "Stage 3: Advanced Transfer & High-Yield Problems",
    subtitle: "High-yield interview standards, complex invariants, and rapid independent derivation.",
  },
};

type ViewMode = "pattern" | "excel" | "stage";

export default function RoadmapDetailPage({ params }: { params: Promise<{ slug: string }> }) {
  const router = useRouter();
  const resolvedParams = use(params);
  const slug = resolvedParams.slug;

  const { user } = useAuth();
  const [roadmap, setRoadmap] = useState<Roadmap | null>(null);
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activating, setActivating] = useState(false);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  // View Arrangement & Filters
  const [viewMode, setViewMode] = useState<ViewMode>("pattern");
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedPatternFilter, setSelectedPatternFilter] = useState<string>("ALL");
  const [collapsedPatterns, setCollapsedPatterns] = useState<Record<string, boolean>>({});

  // Canonical Progress Map
  const [progressMap, setProgressMap] = useState<Record<string, UserQuestionProgress>>({});
  const [activeModalItem, setActiveModalItem] = useState<RoadmapItem | null>(null);
  const [isSubmittingQuickMark, setIsSubmittingQuickMark] = useState(false);

  // Load roadmap & user profile
  useEffect(() => {
    let isMounted = true;
    async function loadData() {
      try {
        const [rData, prof] = await Promise.all([
          api.getRoadmap(slug),
          api.getProfile().catch(() => null),
        ]);
        if (isMounted) {
          setRoadmap(rData);
          setProfile(prof);

          // Automatically default to "stage" view if system template has multiple stages
          const stages = new Set((rData.items || []).map((it) => it.stage_number));
          if (stages.size > 1 && rData.is_system_template) {
            setViewMode("stage");
          } else {
            setViewMode("pattern");
          }
        }
      } catch (err: unknown) {
        const e = err as Error;
        if (isMounted) setError(e.message || "Failed to load roadmap details");
      } finally {
        if (isMounted) setLoading(false);
      }
    }
    loadData();
    return () => {
      isMounted = false;
    };
  }, [slug]);

  // Load canonical progress for user
  useEffect(() => {
    if (!user) return;
    api
      .listAllUserProgress()
      .then((res) => {
        const map: Record<string, UserQuestionProgress> = {};
        (res.progress || []).forEach((p) => {
          map[p.question_id] = p;
        });
        setProgressMap(map);
      })
      .catch(() => {});
  }, [user]);

  const handleSetActive = async () => {
    if (!roadmap) return;
    setActivating(true);
    setStatusMessage(null);
    try {
      const updated = await api.updateProfile({
        selected_roadmap_id: roadmap.id,
      });
      setProfile(updated);
      setStatusMessage("This roadmap is now your active preparation track!");
      setTimeout(() => setStatusMessage(null), 3000);
    } catch (err: unknown) {
      const e = err as Error;
      setStatusMessage(`Failed to set active roadmap: ${e.message}`);
    } finally {
      setActivating(false);
    }
  };

  const handleCheckboxClick = async (item: RoadmapItem) => {
    if (!user) {
      router.push("/login");
      return;
    }

    const currentProg = progressMap[item.question_id];
    const isSolved = currentProg?.status === "SOLVED" || currentProg?.status === "MASTERED";

    if (isSolved) {
      try {
        await api.unmarkProgress(item.question_id);
        setProgressMap((prev) => {
          const updated = { ...prev };
          delete updated[item.question_id];
          return updated;
        });
      } catch (err) {
        console.error("Failed to unmark question:", err);
      }
    } else {
      setActiveModalItem(item);
    }
  };

  const handleModalSubmit = async (data: {
    outcome: AttemptOutcome;
    difficulty_experienced: PerceivedDifficulty;
    needs_revision: boolean;
    confidence_first_try: boolean;
    notes: string;
  }) => {
    if (!activeModalItem) return;
    setIsSubmittingQuickMark(true);
    try {
      const res = await api.quickMarkProgress({
        question_id: activeModalItem.question_id,
        outcome: data.outcome,
        difficulty_experienced: data.difficulty_experienced,
        needs_revision: data.needs_revision,
        confidence_first_try: data.confidence_first_try,
        notes: data.notes,
      });

      if (res.progress) {
        setProgressMap((prev) => ({
          ...prev,
          [activeModalItem.question_id]: res.progress,
        }));
      }
      setActiveModalItem(null);
    } catch (err) {
      console.error("Failed to quick mark question:", err);
    } finally {
      setIsSubmittingQuickMark(false);
    }
  };

  const toggleCollapsePattern = (patternName: string) => {
    setCollapsedPatterns((prev) => ({
      ...prev,
      [patternName]: !prev[patternName],
    }));
  };

  // Distinct Patterns list for filter dropdown
  const uniquePatterns = useMemo(() => {
    if (!roadmap?.items) return [];
    const set = new Set<string>();
    roadmap.items.forEach((it) => {
      const p = it.pattern_name || "General";
      set.add(p);
    });
    return Array.from(set).sort();
  }, [roadmap]);

  // Has multiple stages check
  const hasMultipleStages = useMemo(() => {
    if (!roadmap?.items) return false;
    const stages = new Set(roadmap.items.map((i) => i.stage_number));
    return stages.size > 1;
  }, [roadmap]);

  // Filtered items based on search and pattern dropdown
  const filteredItems = useMemo(() => {
    if (!roadmap?.items) return [];
    let items = [...roadmap.items];

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim();
      items = items.filter(
        (it) =>
          it.question_title.toLowerCase().includes(q) ||
          (it.subpattern_name && it.subpattern_name.toLowerCase().includes(q)) ||
          (it.pattern_name && it.pattern_name.toLowerCase().includes(q))
      );
    }

    if (selectedPatternFilter !== "ALL") {
      items = items.filter(
        (it) => (it.pattern_name || "General").toLowerCase() === selectedPatternFilter.toLowerCase()
      );
    }

    return items;
  }, [roadmap, searchQuery, selectedPatternFilter]);

  // Group items by pattern (preserves order of appearance)
  const patternGroups = useMemo(() => {
    const groups: { patternName: string; items: RoadmapItem[] }[] = [];
    const indexMap = new Map<string, number>();

    filteredItems.forEach((it) => {
      const pName = it.pattern_name || "General";
      if (!indexMap.has(pName)) {
        indexMap.set(pName, groups.length);
        groups.push({ patternName: pName, items: [] });
      }
      const idx = indexMap.get(pName)!;
      groups[idx].items.push(it);
    });

    return groups;
  }, [filteredItems]);

  // Flat list sorted by Excel Order / order index
  const excelOrderedItems = useMemo(() => {
    return [...filteredItems].sort((a, b) => {
      if (a.excel_order && b.excel_order && a.excel_order !== b.excel_order) {
        return a.excel_order - b.excel_order;
      }
      return a.order_index - b.order_index;
    });
  }, [filteredItems]);

  // Group items by stage (1, 2, 3)
  const stageGroups = useMemo(() => {
    const groups: Record<number, RoadmapItem[]> = { 1: [], 2: [], 3: [] };
    filteredItems.forEach((it) => {
      const s = it.stage_number || 1;
      if (!groups[s]) groups[s] = [];
      groups[s].push(it);
    });

    Object.keys(groups).forEach((s) => {
      groups[Number(s)].sort((a, b) => a.order_index - b.order_index);
    });

    return groups;
  }, [filteredItems]);

  // Overall roadmap progress
  const totalQuestions = roadmap?.items?.length || 0;
  const solvedInRoadmap = useMemo(() => {
    if (!roadmap?.items) return 0;
    return roadmap.items.filter((it) => {
      const p = progressMap[it.question_id];
      return p?.status === "SOLVED" || p?.status === "MASTERED";
    }).length;
  }, [roadmap, progressMap]);

  const completionPercent = totalQuestions > 0 ? Math.round((solvedInRoadmap / totalQuestions) * 100) : 0;

  if (loading) {
    return (
      <div className="p-6 md:p-10 max-w-7xl mx-auto w-full space-y-8 animate-pulse min-h-screen">
        <div className="h-6 bg-white/[0.04] rounded-lg w-28 border border-white/[0.06]" />
        <div className="h-44 bg-[#12141a]/60 rounded-2xl border border-white/[0.06]" />
        <div className="space-y-4">
          {[1, 2].map((i) => (
            <div key={i} className="h-48 bg-[#12141a]/60 rounded-2xl border border-white/[0.06]" />
          ))}
        </div>
      </div>
    );
  }

  if (error || !roadmap) {
    return (
      <div className="max-w-2xl mx-auto px-4 py-20 text-center space-y-4">
        <h2 className="text-xl font-bold text-white">Roadmap Not Found</h2>
        <p className="text-xs text-neutral-400">{error || "Could not locate this roadmap curriculum."}</p>
        <Link
          href="/roadmaps"
          className="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-bold bg-gradient-to-r from-amber-500 to-amber-600 text-black shadow-lg shadow-amber-500/20"
        >
          <ArrowLeft className="h-4 w-4" /> Back to Roadmaps
        </Link>
      </div>
    );
  }

  const isActive = profile?.selected_roadmap_id === roadmap.id;

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto w-full space-y-8 text-white min-h-screen">
      {/* Back Link */}
      <Link
        href="/roadmaps"
        className="inline-flex items-center gap-1.5 text-xs font-mono text-amber-400 hover:text-amber-300 transition-colors"
      >
        <ArrowLeft className="h-3.5 w-3.5" /> Back to All Roadmaps
      </Link>

      {/* Header Banner Glass Card */}
      <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.12),0_12px_32px_rgba(0,0,0,0.45)] relative overflow-hidden space-y-4">
        <div className="absolute top-0 right-0 w-80 h-40 bg-amber-500/10 rounded-full blur-3xl pointer-events-none" />
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 relative z-10">
          <div>
            <div className="flex items-center gap-2 mb-2">
              <span className="px-2.5 py-0.5 rounded text-[10px] font-mono font-bold uppercase tracking-wider bg-gradient-to-r from-amber-400 to-amber-500 text-black shadow-sm">
                {roadmap.is_system_template ? "CANONICAL CURRICULUM" : "CUSTOM ROADMAP"}
              </span>
              {roadmap.target_days && (
                <span className="text-xs font-mono text-amber-400 font-semibold">
                  {roadmap.target_days}-Day Plan
                </span>
              )}
            </div>
            <h1 className="text-2xl md:text-3xl font-extrabold tracking-tight text-white">
              {roadmap.title}
            </h1>
            <p className="text-xs md:text-sm text-neutral-400 mt-1 max-w-2xl">
              {roadmap.description || "Pedagogically sequenced DSA problems derived from the Master Question Bank."}
            </p>
          </div>

          <div className="flex items-center gap-3 self-start md:self-auto">
            {isActive ? (
              <span className="px-3.5 py-1.5 rounded-xl bg-amber-500/15 text-amber-300 border border-amber-500/40 font-mono text-xs font-bold flex items-center gap-1.5 shadow-[0_0_15px_rgba(245,158,11,0.15)]">
                <Check className="h-4 w-4 stroke-[3]" /> ACTIVE TRACK
              </span>
            ) : user ? (
              <button
                onClick={handleSetActive}
                disabled={activating}
                className="px-4 py-2 rounded-xl text-xs font-bold bg-gradient-to-r from-amber-500 to-amber-600 text-black shadow-[0_0_18px_rgba(245,158,11,0.25)] hover:shadow-[0_0_28px_rgba(245,158,11,0.4)] transition-all active:scale-[0.98] cursor-pointer"
              >
                {activating ? "Activating..." : "Set as Active Roadmap"}
              </button>
            ) : null}
          </div>
        </div>

        {statusMessage && (
          <div className="p-3 rounded-xl border border-amber-500/30 bg-amber-500/10 text-xs text-amber-200">
            {statusMessage}
          </div>
        )}

        {/* Progress Meter Bar */}
        <div className="pt-3 border-t border-white/[0.06] space-y-2 relative z-10">
          <div className="flex justify-between items-center text-xs font-mono">
            <span className="text-neutral-400">
              Curriculum Progress: <span className="text-white font-bold">{solvedInRoadmap}</span> / {totalQuestions} Solved
            </span>
            <span className="text-base font-extrabold text-amber-400">{completionPercent}%</span>
          </div>
          <div className="h-1.5 w-full rounded-full bg-neutral-900 border border-white/[0.06] overflow-hidden">
            <div
              className="h-full rounded-full bg-gradient-to-r from-amber-400 to-amber-500 shadow-[0_0_12px_rgba(245,158,11,0.5)] transition-all duration-700"
              style={{ width: `${Math.max(2, completionPercent)}%` }}
            />
          </div>
        </div>
      </div>

      {/* Control Bar: View Arrangement Switcher + Pattern Filter + Search */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 p-3.5 rounded-2xl bg-[#0e111a]/80 backdrop-blur-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)]">
        {/* Arrangement Mode Switcher */}
        <div className="flex items-center gap-1.5 bg-[#080a10]/80 p-1 rounded-xl border border-white/[0.06]">
          <button
            onClick={() => setViewMode("pattern")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer ${
              viewMode === "pattern"
                ? "bg-gradient-to-r from-amber-500 to-amber-600 text-black font-bold shadow-sm"
                : "text-neutral-400 hover:text-white"
            }`}
          >
            <Layers className="h-3.5 w-3.5" />
            Categorized by Pattern
          </button>

          <button
            onClick={() => setViewMode("excel")}
            className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer ${
              viewMode === "excel"
                ? "bg-gradient-to-r from-amber-500 to-amber-600 text-black font-bold shadow-sm"
                : "text-neutral-400 hover:text-white"
            }`}
          >
            <ListOrdered className="h-3.5 w-3.5" />
            Excel Sheet Order
          </button>

          {hasMultipleStages && (
            <button
              onClick={() => setViewMode("stage")}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold transition-all cursor-pointer ${
                viewMode === "stage"
                  ? "bg-gradient-to-r from-amber-500 to-amber-600 text-black font-bold shadow-sm"
                  : "text-neutral-400 hover:text-white"
              }`}
            >
              <Sparkles className="h-3.5 w-3.5" />
              3-Stage Curriculum
            </button>
          )}
        </div>

        {/* Filters: Pattern Dropdown & Search Bar */}
        <div className="flex flex-wrap items-center gap-2.5">
          {/* Pattern Filter */}
          <div className="relative">
            <select
              value={selectedPatternFilter}
              onChange={(e) => setSelectedPatternFilter(e.target.value)}
              className="px-3 py-1.5 pr-8 rounded-xl bg-[#121520] border border-white/[0.1] text-xs text-neutral-200 focus:outline-none focus:border-amber-400/50 appearance-none cursor-pointer"
            >
              <option value="ALL">All Patterns ({uniquePatterns.length})</option>
              {uniquePatterns.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
            <ChevronDown className="h-3.5 w-3.5 text-neutral-400 absolute right-2.5 top-1/2 -translate-y-1/2 pointer-events-none" />
          </div>

          {/* Search Input */}
          <div className="relative">
            <Search className="h-3.5 w-3.5 text-neutral-500 absolute left-3 top-1/2 -translate-y-1/2" />
            <input
              type="text"
              placeholder="Search problems..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-8 pr-3 py-1.5 rounded-xl bg-[#121520] border border-white/[0.1] text-xs text-white placeholder-neutral-500 focus:outline-none focus:border-amber-400/50 w-44 sm:w-56"
            />
          </div>
        </div>
      </div>

      {/* VIEW MODE 1: Categorized by Pattern */}
      {viewMode === "pattern" && (
        <div className="space-y-6">
          {patternGroups.length === 0 ? (
            <div className="p-12 text-center rounded-2xl bg-[#0e1017]/50 border border-white/[0.06] text-neutral-400 text-xs">
              No problems match your current search or pattern filter.
            </div>
          ) : (
            patternGroups.map((group) => {
              const isCollapsed = collapsedPatterns[group.patternName];
              const groupSolved = group.items.filter((it) => {
                const p = progressMap[it.question_id];
                return p?.status === "SOLVED" || p?.status === "MASTERED";
              }).length;
              const groupPct = group.items.length > 0 ? Math.round((groupSolved / group.items.length) * 100) : 0;

              return (
                <div
                  key={group.patternName}
                  className="rounded-2xl border border-white/[0.08] bg-[#0e1017]/75 backdrop-blur-2xl overflow-hidden shadow-[inset_0_1px_1px_rgba(255,255,255,0.08),0_12px_32px_rgba(0,0,0,0.35)] transition-all"
                >
                  {/* Pattern Header */}
                  <div
                    onClick={() => toggleCollapsePattern(group.patternName)}
                    className="p-4 bg-[#0a0c12]/90 border-b border-white/[0.06] flex items-center justify-between cursor-pointer hover:bg-white/[0.02] transition-colors select-none"
                  >
                    <div className="flex items-center gap-3">
                      <div className="text-amber-400">
                        {isCollapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronDown className="h-4 w-4" />}
                      </div>
                      <div className="flex items-center gap-2.5">
                        <h2 className="text-sm font-bold text-white tracking-wide">
                          {group.patternName}
                        </h2>
                        <span className="text-[10px] font-mono text-neutral-400 px-2 py-0.5 rounded-full bg-white/[0.05] border border-white/[0.08]">
                          {group.items.length} {group.items.length === 1 ? "problem" : "problems"}
                        </span>
                      </div>
                    </div>

                    <div className="flex items-center gap-3">
                      <span className="text-xs font-mono text-amber-400 font-semibold">
                        {groupSolved} / {group.items.length} Solved ({groupPct}%)
                      </span>
                    </div>
                  </div>

                  {/* Problem Table */}
                  {!isCollapsed && (
                    <div className="overflow-x-auto">
                      <table className="w-full text-left text-xs border-collapse">
                        <thead>
                          <tr className="border-b border-white/[0.06] bg-black/30 text-neutral-400 font-mono uppercase text-[10.5px]">
                            <th className="py-2.5 px-4 w-12 text-center">Done</th>
                            <th className="py-2.5 px-3 w-16 text-center">#</th>
                            <th className="py-2.5 px-4">Problem</th>
                            <th className="py-2.5 px-4 w-48">Subpattern</th>
                            <th className="py-2.5 px-3 w-24 text-center">Difficulty</th>
                            <th className="py-2.5 px-4 w-28 text-center">Workspace</th>
                          </tr>
                        </thead>
                        <tbody className="divide-y divide-white/[0.04]">
                          {group.items.map((item, idx) => {
                            const prog = progressMap[item.question_id];
                            const isSolved = prog?.status === "SOLVED" || prog?.status === "MASTERED";
                            const isMastered = prog?.status === "MASTERED";

                            return (
                              <tr
                                key={item.id}
                                className={`hover:bg-white/[0.03] transition-colors ${
                                  isSolved ? "bg-amber-500/[0.04]" : ""
                                }`}
                              >
                                {/* Checkbox */}
                                <td className="py-3 px-4 text-center">
                                  <button
                                    type="button"
                                    onClick={() => handleCheckboxClick(item)}
                                    className={`h-5 w-5 rounded-md border flex items-center justify-center transition-all cursor-pointer ${
                                      isSolved
                                        ? "bg-gradient-to-r from-amber-400 to-amber-500 border-amber-400 text-black shadow-[0_0_10px_rgba(245,158,11,0.4)]"
                                        : "border-white/[0.15] bg-[#14161f]/60 hover:border-amber-400/60"
                                    }`}
                                    title={isSolved ? "Marked as solved (Click to unmark)" : "Log completion"}
                                  >
                                    {isSolved && <Check className="h-3.5 w-3.5 stroke-[3]" />}
                                  </button>
                                </td>

                                {/* Sequential / Excel Order */}
                                <td className="py-3 px-3 text-center font-mono text-[11px] text-neutral-400">
                                  {item.excel_order || idx + 1}
                                </td>

                                {/* Problem Title */}
                                <td className="py-3 px-4 font-medium text-white">
                                  <div className="flex items-center gap-2">
                                    <Link
                                      href={`/questions/${item.question_id}`}
                                      className="hover:text-amber-400 hover:underline transition-colors"
                                    >
                                      {item.question_title}
                                    </Link>
                                    {isMastered && (
                                      <span className="text-[10px] font-mono font-bold text-amber-300 border border-amber-400/40 px-1 rounded bg-amber-500/10">
                                        ★ MASTERED
                                      </span>
                                    )}
                                  </div>
                                </td>

                                {/* Subpattern Badge */}
                                <td className="py-3 px-4">
                                  {item.subpattern_name ? (
                                    <span className="px-2 py-0.5 rounded text-[10.5px] font-mono text-neutral-300 bg-white/[0.04] border border-white/[0.08]">
                                      {item.subpattern_name}
                                    </span>
                                  ) : (
                                    <span className="text-neutral-600 font-mono text-[10px]">&mdash;</span>
                                  )}
                                </td>

                                {/* Difficulty */}
                                <td className="py-3 px-3 text-center">
                                  <span
                                    className={`px-2 py-0.5 rounded font-mono text-[10px] uppercase font-bold border ${
                                      item.difficulty === "EASY"
                                        ? "text-neutral-300 border-white/20 bg-white/[0.03]"
                                        : item.difficulty === "MEDIUM"
                                        ? "text-amber-400 border-amber-500/30 bg-amber-500/10"
                                        : "text-amber-300 border-amber-400/60 bg-amber-500/20 font-extrabold"
                                    }`}
                                  >
                                    {item.difficulty}
                                  </span>
                                </td>

                                {/* Solve CTA */}
                                <td className="py-3 px-4 text-center">
                                  <Link
                                    href={`/questions/${item.question_id}`}
                                    className="inline-flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-bold text-black bg-gradient-to-r from-amber-400 to-amber-500 hover:shadow-[0_0_12px_rgba(245,158,11,0.4)] transition-all cursor-pointer"
                                  >
                                    <BookOpen className="h-3 w-3 stroke-[2.5]" />
                                    Solve
                                  </Link>
                                </td>
                              </tr>
                            );
                          })}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>
      )}

      {/* VIEW MODE 2: Excel Sheet Order (Preserves Raw Spreadsheet Row Order) */}
      {viewMode === "excel" && (
        <div className="rounded-2xl border border-white/[0.08] bg-[#0e1017]/75 backdrop-blur-2xl overflow-hidden shadow-[inset_0_1px_1px_rgba(255,255,255,0.08),0_12px_32px_rgba(0,0,0,0.35)]">
          <div className="p-4 bg-[#0a0c12]/90 border-b border-white/[0.06] flex items-center justify-between">
            <div className="flex items-center gap-2">
              <ListOrdered className="h-4 w-4 text-amber-400" />
              <h2 className="text-sm font-bold text-white tracking-wide">
                Spreadsheet Sequence ({excelOrderedItems.length} problems)
              </h2>
            </div>
            <span className="text-xs font-mono text-amber-400 font-semibold">
              Sorted by Excel ID / Row Order
            </span>
          </div>

          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs border-collapse">
              <thead>
                <tr className="border-b border-white/[0.08] bg-[#0a0c12]/90 text-neutral-400 font-mono uppercase text-[10.5px]">
                  <th className="py-3 px-4 w-12 text-center">Done</th>
                  <th className="py-3 px-3 w-16 text-center">ID</th>
                  <th className="py-3 px-4">Problem</th>
                  <th className="py-3 px-4 w-44">Pattern</th>
                  <th className="py-3 px-4 w-44">Subpattern</th>
                  <th className="py-3 px-3 w-24 text-center">Difficulty</th>
                  <th className="py-3 px-4 w-28 text-center">Workspace</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-white/[0.04]">
                {excelOrderedItems.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="py-8 text-center text-neutral-400 text-xs">
                      No questions match your current filters.
                    </td>
                  </tr>
                ) : (
                  excelOrderedItems.map((item, idx) => {
                    const prog = progressMap[item.question_id];
                    const isSolved = prog?.status === "SOLVED" || prog?.status === "MASTERED";
                    const isMastered = prog?.status === "MASTERED";

                    return (
                      <tr
                        key={item.id}
                        className={`hover:bg-white/[0.03] transition-colors ${
                          isSolved ? "bg-amber-500/[0.04]" : ""
                        }`}
                      >
                        {/* Checkbox */}
                        <td className="py-3.5 px-4 text-center">
                          <button
                            type="button"
                            onClick={() => handleCheckboxClick(item)}
                            className={`h-5 w-5 rounded-md border flex items-center justify-center transition-all cursor-pointer ${
                              isSolved
                                ? "bg-gradient-to-r from-amber-400 to-amber-500 border-amber-400 text-black shadow-[0_0_10px_rgba(245,158,11,0.4)]"
                                : "border-white/[0.15] bg-[#14161f]/60 hover:border-amber-400/60"
                            }`}
                            title={isSolved ? "Marked as solved (Click to unmark)" : "Log completion"}
                          >
                            {isSolved && <Check className="h-3.5 w-3.5 stroke-[3]" />}
                          </button>
                        </td>

                        {/* Excel ID */}
                        <td className="py-3.5 px-3 text-center font-mono text-[11px] text-amber-400/90 font-semibold">
                          #{item.excel_order || idx + 1}
                        </td>

                        {/* Problem Title */}
                        <td className="py-3.5 px-4 font-medium text-white">
                          <div className="flex items-center gap-2">
                            <Link
                              href={`/questions/${item.question_id}`}
                              className="hover:text-amber-400 hover:underline transition-colors"
                            >
                              {item.question_title}
                            </Link>
                            {isMastered && (
                              <span className="text-[10px] font-mono font-bold text-amber-300 border border-amber-400/40 px-1 rounded bg-amber-500/10">
                                ★ MASTERED
                              </span>
                            )}
                          </div>
                        </td>

                        {/* Pattern */}
                        <td className="py-3.5 px-4">
                          <span className="text-neutral-200 text-xs font-medium">
                            {item.pattern_name || "General"}
                          </span>
                        </td>

                        {/* Subpattern */}
                        <td className="py-3.5 px-4">
                          {item.subpattern_name ? (
                            <span className="px-2 py-0.5 rounded text-[10.5px] font-mono text-neutral-300 bg-white/[0.04] border border-white/[0.08]">
                              {item.subpattern_name}
                            </span>
                          ) : (
                            <span className="text-neutral-600 font-mono text-[10px]">&mdash;</span>
                          )}
                        </td>

                        {/* Difficulty */}
                        <td className="py-3.5 px-3 text-center">
                          <span
                            className={`px-2 py-0.5 rounded font-mono text-[10px] uppercase font-bold border ${
                              item.difficulty === "EASY"
                                ? "text-neutral-300 border-white/20 bg-white/[0.03]"
                                : item.difficulty === "MEDIUM"
                                ? "text-amber-400 border-amber-500/30 bg-amber-500/10"
                                : "text-amber-300 border-amber-400/60 bg-amber-500/20 font-extrabold"
                            }`}
                          >
                            {item.difficulty}
                          </span>
                        </td>

                        {/* Solve CTA */}
                        <td className="py-3.5 px-4 text-center">
                          <Link
                            href={`/questions/${item.question_id}`}
                            className="inline-flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-bold text-black bg-gradient-to-r from-amber-400 to-amber-500 hover:shadow-[0_0_12px_rgba(245,158,11,0.4)] transition-all cursor-pointer"
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
        </div>
      )}

      {/* VIEW MODE 3: 3-Stage Curriculum */}
      {viewMode === "stage" && (
        <div className="space-y-8">
          {[1, 2, 3].map((stageNum) => {
            const items = stageGroups[stageNum] || [];
            if (items.length === 0) return null;

            const stageInfo = STAGE_CONFIG[stageNum] || {
              title: `Stage ${stageNum}`,
              subtitle: "Targeted problem set",
            };

            const stageSolved = items.filter((it) => {
              const p = progressMap[it.question_id];
              return p?.status === "SOLVED" || p?.status === "MASTERED";
            }).length;

            return (
              <div key={stageNum} className="space-y-4">
                {/* Stage Header */}
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-white/[0.08] pb-3">
                  <div>
                    <h2 className="text-base font-bold text-white flex items-center gap-2.5">
                      <span className="h-6 w-6 rounded-md bg-gradient-to-r from-amber-400 to-amber-500 text-black font-extrabold text-xs flex items-center justify-center shadow-sm">
                        {stageNum}
                      </span>
                      {stageInfo.title}
                    </h2>
                    <p className="text-xs text-neutral-400 mt-0.5">{stageInfo.subtitle}</p>
                  </div>
                  <span className="text-xs font-mono text-amber-400 font-semibold">
                    {stageSolved} / {items.length} Completed
                  </span>
                </div>

                {/* Stage Problems Glass Table */}
                <div className="rounded-2xl border border-white/[0.08] bg-[#0e1017]/75 backdrop-blur-2xl overflow-hidden shadow-[inset_0_1px_1px_rgba(255,255,255,0.08),0_12px_32px_rgba(0,0,0,0.35)]">
                  <table className="w-full text-left text-xs border-collapse">
                    <thead>
                      <tr className="border-b border-white/[0.08] bg-[#0a0c12]/90 text-neutral-400 font-mono uppercase text-[11px]">
                        <th className="py-3 px-4 w-12 text-center">Done</th>
                        <th className="py-3 px-3 w-16 text-center">#</th>
                        <th className="py-3 px-4">Problem</th>
                        <th className="py-3 px-4 w-40">Pattern</th>
                        <th className="py-3 px-3 w-24 text-center">Difficulty</th>
                        <th className="py-3 px-4 w-28 text-center">Workspace</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-white/[0.04]">
                      {items.map((item, idx) => {
                        const prog = progressMap[item.question_id];
                        const isSolved = prog?.status === "SOLVED" || prog?.status === "MASTERED";
                        const isMastered = prog?.status === "MASTERED";

                        return (
                          <tr
                            key={item.id}
                            className={`hover:bg-white/[0.03] transition-colors ${
                              isSolved ? "bg-amber-500/[0.04]" : ""
                            }`}
                          >
                            {/* Checkbox */}
                            <td className="py-3.5 px-4 text-center">
                              <button
                                type="button"
                                onClick={() => handleCheckboxClick(item)}
                                className={`h-5 w-5 rounded-md border flex items-center justify-center transition-all cursor-pointer ${
                                  isSolved
                                    ? "bg-gradient-to-r from-amber-400 to-amber-500 border-amber-400 text-black shadow-[0_0_10px_rgba(245,158,11,0.4)]"
                                    : "border-white/[0.15] bg-[#14161f]/60 hover:border-amber-400/60"
                                }`}
                                title={isSolved ? "Marked as solved (Click to unmark)" : "Log completion"}
                              >
                                {isSolved && <Check className="h-3.5 w-3.5 stroke-[3]" />}
                              </button>
                            </td>

                            {/* Order */}
                            <td className="py-3.5 px-3 text-center font-mono text-[11px] text-neutral-400">
                              {item.excel_order || idx + 1}
                            </td>

                            {/* Problem Title */}
                            <td className="py-3.5 px-4 font-medium text-white">
                              <div className="flex items-center gap-2">
                                <Link
                                  href={`/questions/${item.question_id}`}
                                  className="hover:text-amber-400 hover:underline transition-colors"
                                >
                                  {item.question_title}
                                </Link>
                                {isMastered && (
                                  <span className="text-[10px] font-mono font-bold text-amber-300 border border-amber-400/40 px-1 rounded bg-amber-500/10">
                                    ★ MASTERED
                                  </span>
                                )}
                              </div>
                            </td>

                            {/* Pattern */}
                            <td className="py-3.5 px-4">
                              <span className="text-neutral-300 text-xs">
                                {item.pattern_name || "General"}
                              </span>
                            </td>

                            {/* Difficulty */}
                            <td className="py-3.5 px-3 text-center">
                              <span
                                className={`px-2 py-0.5 rounded font-mono text-[10px] uppercase font-bold border ${
                                  item.difficulty === "EASY"
                                    ? "text-neutral-300 border-white/20 bg-white/[0.03]"
                                    : item.difficulty === "MEDIUM"
                                    ? "text-amber-400 border-amber-500/30 bg-amber-500/10"
                                    : "text-amber-300 border-amber-400/60 bg-amber-500/20 font-extrabold"
                                }`}
                              >
                                {item.difficulty}
                              </span>
                            </td>

                            {/* Solve CTA */}
                            <td className="py-3.5 px-4 text-center">
                              <Link
                                href={`/questions/${item.question_id}`}
                                className="inline-flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-bold text-black bg-gradient-to-r from-amber-400 to-amber-500 hover:shadow-[0_0_12px_rgba(245,158,11,0.4)] transition-all cursor-pointer"
                              >
                                <BookOpen className="h-3 w-3 stroke-[2.5]" />
                                Solve
                              </Link>
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Quick Mark Modal */}
      {activeModalItem && (
        <QuickMarkModal
          isOpen={true}
          questionTitle={activeModalItem.question_title}
          questionDifficulty={activeModalItem.difficulty}
          patternName={activeModalItem.pattern_name}
          onClose={() => setActiveModalItem(null)}
          onSubmit={handleModalSubmit}
          isSubmitting={isSubmittingQuickMark}
        />
      )}
    </div>
  );
}
