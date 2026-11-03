"use client";

import React, { useEffect, useState, useCallback } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { api } from "@/lib/api";
import {
  Question,
  UserQuestionProgress,
  QuestionAttempt,
  RevisionSchedule,
  HintResult,
  ReviewResult,
  ExplainResult,
  AIPricingResponse,
  DEFAULT_AI_PRICING,
  AttemptOutcome,
  Difficulty,
} from "@/types";
import {
  Sparkles,
  Clock,
  ExternalLink,
  ChevronLeft,
  CheckCircle2,
  Code2,
  HelpCircle,
  Play,
  Pause,
  RotateCcw,
  Star,
  Tag,
  AlertCircle,
  Send,
  Zap,
} from "lucide-react";

function QuestionDetailContent() {
  const params = useParams();
  const router = useRouter();
  const questionId = params?.id as string;
  const { user, credits, refreshCredits } = useAuth();
  const [pricing, setPricing] = useState<AIPricingResponse>(DEFAULT_AI_PRICING);

  const [question, setQuestion] = useState<Question | null>(null);
  const [progress, setProgress] = useState<UserQuestionProgress | null>(null);
  const [attempts, setAttempts] = useState<QuestionAttempt[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Active AI Tab
  const [activeTab, setActiveTab] = useState<"hints" | "review" | "explain">("hints");

  // AI Hint Ladder state
  const [selectedHintLevel, setSelectedHintLevel] = useState<number>(1);
  const [candidateNotes, setCandidateNotes] = useState("");
  const [hintResult, setHintResult] = useState<HintResult | null>(null);
  const [hintLoading, setHintLoading] = useState(false);
  const [hintError, setHintError] = useState<string | null>(null);

  // AI Review state
  const [candidateCode, setCandidateCode] = useState("");
  const [proposedTime, setProposedTime] = useState("O(N)");
  const [proposedSpace, setProposedSpace] = useState("O(1)");
  const [reviewResult, setReviewResult] = useState<ReviewResult | null>(null);
  const [reviewLoading, setReviewLoading] = useState(false);
  const [reviewError, setReviewError] = useState<string | null>(null);

  // AI Explain state
  const [explainQuery, setExplainQuery] = useState("");
  const [explainResult, setExplainResult] = useState<ExplainResult | null>(null);
  const [explainLoading, setExplainLoading] = useState(false);
  const [explainError, setExplainError] = useState<string | null>(null);

  // Attempt Recording Modal & Stopwatch
  const [showAttemptModal, setShowAttemptModal] = useState(false);
  const [timerSeconds, setTimerSeconds] = useState(0);
  const [isTimerRunning, setIsTimerRunning] = useState(false);
  const [outcome, setOutcome] = useState("SOLVED_INDEPENDENTLY");
  const [perceivedDifficulty, setPerceivedDifficulty] = useState("JUST_RIGHT");
  const [qualityRating, setQualityRating] = useState(4);
  const [selectedMistakeTags, setSelectedMistakeTags] = useState<string[]>([]);
  const [approachSummary, setApproachSummary] = useState("");
  const [attemptCode, setAttemptCode] = useState("");
  const [isLoggingAttempt, setIsLoggingAttempt] = useState(false);
  const [attemptFeedback, setAttemptFeedback] = useState<RevisionSchedule | null>(null);

  // Stopwatch interval
  useEffect(() => {
    let interval: NodeJS.Timeout | null = null;
    if (isTimerRunning) {
      interval = setInterval(() => {
        setTimerSeconds((prev) => prev + 1);
      }, 1000);
    }
    return () => {
      if (interval) clearInterval(interval);
    };
  }, [isTimerRunning]);

  const loadData = useCallback(async () => {
    if (!questionId) return;
    setIsLoading(true);
    setError(null);
    try {
      const [qData, pricingData] = await Promise.all([
        api.getQuestion(questionId),
        api.getAIPricing().catch(() => DEFAULT_AI_PRICING),
      ]);
      setQuestion(qData);
      setPricing(pricingData);

      if (user) {
        try {
          const progData = await api.getQuestionProgress(questionId);
          setProgress(progData.progress);
          setAttempts(progData.attempts || []);
        } catch {
          // Progress might not exist yet
        }
      }
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message || "Failed to load problem details");
    } finally {
      setIsLoading(false);
    }
  }, [questionId, user]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleRequestHint = async (level: number) => {
    if (!user) {
      router.push("/login");
      return;
    }
    setHintLoading(true);
    setHintError(null);
    try {
      const result = await api.requestHint(questionId, level, candidateNotes);
      setHintResult(result);
      if (refreshCredits) refreshCredits();
    } catch (err: unknown) {
      const e = err as Error;
      setHintError(e.message || "Failed to generate hint from AI");
    } finally {
      setHintLoading(false);
    }
  };

  const handleRequestReview = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!user) {
      router.push("/login");
      return;
    }
    setReviewLoading(true);
    setReviewError(null);
    try {
      const result = await api.reviewApproach(
        questionId,
        candidateCode,
        proposedTime,
        proposedSpace
      );
      setReviewResult(result);
      if (refreshCredits) refreshCredits();
    } catch (err: unknown) {
      const e = err as Error;
      setReviewError(e.message || "Failed to submit approach for review");
    } finally {
      setReviewLoading(false);
    }
  };

  const handleRequestExplain = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!user) {
      router.push("/login");
      return;
    }
    setExplainLoading(true);
    setExplainError(null);
    try {
      const patternName = question?.patterns?.[0]?.name || "Algorithms";
      const result = await api.explainConcept(
        "Data Structures & Algorithms",
        patternName,
        explainQuery
      );
      setExplainResult(result);
      if (refreshCredits) refreshCredits();
    } catch (err: unknown) {
      const e = err as Error;
      setExplainError(e.message || "Failed to retrieve concept explanation");
    } finally {
      setExplainLoading(false);
    }
  };

  const handleRecordAttempt = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!user) {
      router.push("/login");
      return;
    }
    setIsLoggingAttempt(true);
    try {
      const res = await api.recordAttempt(questionId, {
        outcome: outcome as AttemptOutcome,
        time_spent_secs: timerSeconds > 0 ? timerSeconds : 900,
        difficulty_experienced: perceivedDifficulty as Difficulty,
        quality_rating: qualityRating,
        mistake_tags: selectedMistakeTags,
        approach_summary: approachSummary || undefined,
        code_snippet: attemptCode || undefined,
      });
      setProgress(res.progress);
      setAttemptFeedback(res.revision || null);
      if (res.attempt) {
        setAttempts((prev) => [res.attempt, ...prev]);
      }
      setIsTimerRunning(false);
    } catch (err: unknown) {
      const e = err as Error;
      alert(`Failed to log attempt: ${e.message}`);
    } finally {
      setIsLoggingAttempt(false);
    }
  };

  const toggleMistakeTag = (tag: string) => {
    setSelectedMistakeTags((prev) =>
      prev.includes(tag) ? prev.filter((t) => t !== tag) : [...prev, tag]
    );
  };

  const formatTimer = (totalSecs: number) => {
    const mins = Math.floor(totalSecs / 60);
    const secs = totalSecs % 60;
    return `${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  };

  if (isLoading) {
    return (
      <div className="p-6 md:p-10 max-w-7xl mx-auto w-full space-y-6 animate-pulse min-h-screen">
        <div className="h-6 bg-amber-500/10 rounded w-1/5" />
        <div className="h-40 bg-[#0e1017]/60 rounded-2xl border border-white/[0.08]" />
        <div className="h-96 bg-[#0e1017]/60 rounded-2xl border border-white/[0.08]" />
      </div>
    );
  }

  if (error || !question) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-16 text-center space-y-4 min-h-screen">
        <AlertCircle className="h-12 w-12 text-amber-400 mx-auto" />
        <h2 className="text-xl font-bold text-white">Problem Not Found</h2>
        <p className="text-sm text-neutral-400">{error || "Unable to retrieve problem details."}</p>
        <Link
          href="/questions"
          className="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-[#0e1017]/80 border border-amber-500/30 hover:border-amber-400 text-xs font-semibold text-amber-400 backdrop-blur-md"
        >
          <ChevronLeft className="h-4 w-4" /> Back to Master Catalog
        </Link>
      </div>
    );
  }

  const hintLadderOptions = [
    { level: 1, name: "Conceptual Nudge", cost: pricing.hints["1"] ?? 2, desc: "High-level direction without giving away the algorithm" },
    { level: 2, name: "Algorithmic Direction", cost: pricing.hints["2"] ?? 2, desc: "Concrete pattern choice (e.g. 2-Pointers vs Dynamic Programming)" },
    { level: 3, name: "Key Data Structure & Invariant", cost: pricing.hints["3"] ?? 4, desc: "Specific invariant, monotonic queue, or hash state" },
    { level: 4, name: "Pseudocode Outline", cost: pricing.hints["4"] ?? 4, desc: "Step-by-step structural pseudocode outline" },
  ];

  const standardMistakeTags = [
    "OFF_BY_ONE",
    "EDGE_CASE_EMPTY",
    "TIME_LIMIT_EXCEEDED",
    "WRONG_DATA_STRUCTURE",
    "INTEGER_OVERFLOW",
    "MEMORY_LIMIT_EXCEEDED",
    "LOGIC_INVERSION",
  ];

  const isSolved = progress?.status === "SOLVED" || progress?.status === "MASTERED";

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto w-full space-y-8 text-white min-h-screen">
      {/* Breadcrumb Navigation */}
      <div className="flex items-center justify-between">
        <Link
          href="/questions"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-neutral-400 hover:text-amber-300 transition-colors font-mono"
        >
          <ChevronLeft className="h-4 w-4" />
          Back to Master Question Bank
        </Link>

        {progress && (
          <div className="flex items-center gap-2 text-xs font-mono">
            <span className="text-neutral-400">Status:</span>
            <span
              className={`px-2 py-0.5 rounded font-bold uppercase ${
                isSolved
                  ? "bg-amber-500/15 text-amber-300 border border-amber-500/40"
                  : "bg-white/[0.04] text-neutral-400 border border-white/10"
              }`}
            >
              {progress.status}
            </span>
          </div>
        )}
      </div>

      {/* Question Header Card */}
      <div className="p-6 sm:p-8 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.12),0_12px_32px_rgba(0,0,0,0.45)] space-y-4 relative overflow-hidden">
        <div className="absolute top-0 right-0 w-80 h-40 bg-amber-500/10 rounded-full blur-3xl pointer-events-none" />
        <div className="flex flex-wrap items-center gap-2 relative z-10">
          <span
            className={`px-2.5 py-0.5 rounded text-[10px] font-bold border uppercase font-mono ${
              question.difficulty === "EASY"
                ? "bg-white/[0.04] text-neutral-300 border-white/20"
                : question.difficulty === "MEDIUM"
                ? "bg-amber-500/10 text-amber-400 border-amber-500/30"
                : "bg-amber-500/20 text-amber-300 border-amber-400/60 font-extrabold"
            }`}
          >
            {question.difficulty}
          </span>
          {question.excel_order && (
            <span className="px-2.5 py-0.5 rounded text-[10px] font-mono bg-white/[0.04] text-neutral-400 border border-white/10">
              Excel #{question.excel_order}
            </span>
          )}
          {question.patterns?.map((p) => (
            <span
              key={p.id}
              className="px-2.5 py-0.5 rounded bg-amber-500/15 text-amber-300 border border-amber-500/30 text-[11px] font-mono"
            >
              {p.name}
            </span>
          ))}
          {question.sample_time_limit_mins && (
            <span className="inline-flex items-center gap-1 text-[11px] text-neutral-400 font-mono ml-auto">
              <Clock className="h-3.5 w-3.5 text-amber-400" />
              Target: {question.sample_time_limit_mins} mins
            </span>
          )}
        </div>

        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 relative z-10">
          <h1 className="text-2xl sm:text-3xl font-extrabold text-white tracking-tight">
            {question.title}
          </h1>

          <div className="flex items-center gap-2 shrink-0">
            {question.external_url && (
              <a
                href={question.external_url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl bg-white/[0.04] hover:bg-white/[0.08] text-neutral-300 hover:text-white text-xs font-semibold border border-white/[0.1] transition-colors"
              >
                <span>LeetCode Problem</span>
                <ExternalLink className="h-3.5 w-3.5 text-amber-400" />
              </a>
            )}

            <button
              onClick={() => setShowAttemptModal(true)}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-xl bg-gradient-to-r from-amber-400 to-amber-500 text-black font-bold text-xs transition-all shadow-[0_0_15px_rgba(245,158,11,0.25)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)]"
            >
              <CheckCircle2 className="h-4 w-4 stroke-[2.5]" />
              <span>Record Attempt</span>
            </button>
          </div>
        </div>

        {question.editorial_summary && (
          <div className="pt-2 border-t border-white/[0.06] relative z-10">
            <p className="text-xs text-neutral-400 leading-relaxed">
              <span className="font-semibold text-amber-400">Algorithmic Note: </span>
              {question.editorial_summary}
            </p>
          </div>
        )}
      </div>

      {/* Practice Workspace: Left is Practice Tools & Stopwatch, Right is AI Tutor Suite */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* Left Column: Stopwatch & Attempt History (4 cols) */}
        <div className="lg:col-span-4 space-y-6">
          {/* Stopwatch Card */}
          <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)] space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-bold text-white flex items-center gap-2">
                <Clock className="h-4 w-4 text-amber-400" />
                Interview Timer
              </h2>
              <span className="text-xs font-mono text-neutral-400">Live Practice</span>
            </div>

            <div className="py-4 text-center">
              <span className="text-4xl font-extrabold font-mono text-white tracking-widest">
                {formatTimer(timerSeconds)}
              </span>
            </div>

            <div className="flex items-center justify-center gap-2">
              <button
                type="button"
                onClick={() => setIsTimerRunning(!isTimerRunning)}
                className={`flex items-center gap-1.5 px-4 py-2 rounded-xl text-xs font-bold transition-all ${
                  isTimerRunning
                    ? "bg-amber-500/20 text-amber-300 border border-amber-500/50"
                    : "bg-gradient-to-r from-amber-400 to-amber-500 text-black shadow-[0_0_15px_rgba(245,158,11,0.25)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)]"
                }`}
              >
                {isTimerRunning ? (
                  <>
                    <Pause className="h-3.5 w-3.5 stroke-[2.5]" /> Pause
                  </>
                ) : (
                  <>
                    <Play className="h-3.5 w-3.5 stroke-[2.5]" /> Start Timer
                  </>
                )}
              </button>
              <button
                type="button"
                onClick={() => {
                  setIsTimerRunning(false);
                  setTimerSeconds(0);
                }}
                className="p-2 rounded-xl bg-white/[0.04] border border-white/[0.1] hover:border-amber-400/50 text-neutral-400 hover:text-white transition-colors"
                title="Reset stopwatch"
              >
                <RotateCcw className="h-4 w-4 text-amber-400" />
              </button>
            </div>
          </div>

          {/* Past Attempts on this problem */}
          <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)] space-y-4">
            <h2 className="text-sm font-bold text-white flex items-center gap-2">
              <CheckCircle2 className="h-4 w-4 text-amber-400" />
              Past Attempts ({attempts.length})
            </h2>

            {attempts.length > 0 ? (
              <div className="space-y-3 max-h-80 overflow-y-auto pr-1">
                {attempts.map((att) => (
                  <div
                    key={att.id}
                    className="p-3 rounded-xl bg-[#121520]/60 border border-white/[0.06] text-xs space-y-1.5"
                  >
                    <div className="flex items-center justify-between">
                      <span
                        className={`font-mono text-[10px] font-semibold uppercase px-1.5 py-0.5 rounded ${
                          att.outcome === "SOLVED_INDEPENDENTLY"
                            ? "bg-amber-500/15 text-amber-300 border border-amber-500/30 font-bold"
                            : "bg-white/[0.04] text-neutral-400 border border-white/10"
                        }`}
                      >
                        {att.outcome.replace(/_/g, " ")}
                      </span>
                      <span className="text-neutral-500 text-[10px] font-mono">
                        {new Date(att.created_at).toLocaleDateString()}
                      </span>
                    </div>

                    <div className="flex items-center justify-between text-neutral-400 font-mono text-[11px]">
                      <span>{Math.round(att.time_spent_secs / 60)} mins</span>
                      <span>Quality: {att.quality_rating} / 5</span>
                    </div>

                    {att.mistake_tags && att.mistake_tags.length > 0 && (
                      <div className="flex flex-wrap gap-1 pt-1">
                        {att.mistake_tags.map((t) => (
                          <span
                            key={t}
                            className="px-1.5 py-0.5 rounded bg-amber-500/10 text-[9px] font-mono text-amber-400 border border-amber-500/30"
                          >
                            #{t}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <p className="text-xs text-neutral-500 font-mono">
                No past attempts recorded yet.
              </p>
            )}
          </div>
        </div>

        {/* Right Column: AI Tutoring Suite (8 cols) */}
        <div className="lg:col-span-8 space-y-6">
          {/* Navigation Tabs */}
          <div className="flex items-center gap-2 p-1.5 bg-[#0e1017]/75 backdrop-blur-2xl rounded-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)]">
            <button
              onClick={() => setActiveTab("hints")}
              className={`flex-1 flex items-center justify-center gap-2 py-2.5 rounded-xl text-xs font-bold transition-all ${
                activeTab === "hints"
                  ? "bg-gradient-to-r from-amber-400 to-amber-500 text-black shadow-[0_0_15px_rgba(245,158,11,0.25)]"
                  : "text-neutral-400 hover:text-white"
              }`}
            >
              <Sparkles className="h-3.5 w-3.5" />
              Progressive Hint Ladder
            </button>
            <button
              onClick={() => setActiveTab("review")}
              className={`flex-1 flex items-center justify-center gap-2 py-2.5 rounded-xl text-xs font-bold transition-all ${
                activeTab === "review"
                  ? "bg-gradient-to-r from-amber-400 to-amber-500 text-black shadow-[0_0_15px_rgba(245,158,11,0.25)]"
                  : "text-neutral-400 hover:text-white"
              }`}
            >
              <Code2 className="h-3.5 w-3.5" />
              Approach &amp; Code Review
            </button>
            <button
              onClick={() => setActiveTab("explain")}
              className={`flex-1 flex items-center justify-center gap-2 py-2.5 rounded-xl text-xs font-bold transition-all ${
                activeTab === "explain"
                  ? "bg-gradient-to-r from-amber-400 to-amber-500 text-black shadow-[0_0_15px_rgba(245,158,11,0.25)]"
                  : "text-neutral-400 hover:text-white"
              }`}
            >
              <HelpCircle className="h-3.5 w-3.5" />
              Concept Q&amp;A
            </button>
          </div>

          {/* Tab 1: Progressive Hint Ladder */}
          {activeTab === "hints" && (
            <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)] space-y-6">
              <div>
                <h2 className="text-base font-bold text-white flex items-center gap-2">
                  <Sparkles className="h-5 w-5 text-amber-400" />
                  Socratic Progressive Hints
                </h2>
                <p className="text-xs text-neutral-400 mt-1">
                  Choose the level of guidance you need. Lower tiers preserve your independent
                  learning retention.
                </p>
              </div>

              {/* Ladder Selection Grid */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {hintLadderOptions.map((opt) => (
                  <button
                    key={opt.level}
                    type="button"
                    onClick={() => setSelectedHintLevel(opt.level)}
                    className={`p-4 rounded-xl text-left border transition-all ${
                      selectedHintLevel === opt.level
                        ? "bg-amber-500/15 border-amber-400 text-white shadow-[0_0_15px_rgba(245,158,11,0.2)]"
                        : "bg-[#10131e]/50 border-white/[0.08] hover:border-amber-400/40 text-neutral-300"
                    }`}
                  >
                    <div className="flex items-center justify-between mb-1.5">
                      <span className="text-xs font-bold text-white font-mono">
                        Tier {opt.level}: {opt.name}
                      </span>
                      <span className="px-2 py-0.5 rounded text-[10px] font-bold font-mono bg-amber-500/10 text-amber-300 border border-amber-500/30">
                        {opt.cost} {opt.cost === 1 ? "credit" : "credits"}
                      </span>
                    </div>
                    <p className="text-[11px] text-neutral-400 leading-snug">{opt.desc}</p>
                  </button>
                ))}
              </div>

              {/* Optional Candidate Notes */}
              <div>
                <label className="block text-xs font-medium text-neutral-300 mb-1.5 uppercase font-mono">
                  Your Current Thoughts / Stuck Point (Optional)
                </label>
                <textarea
                  value={candidateNotes}
                  onChange={(e) => setCandidateNotes(e.target.value)}
                  placeholder="e.g. I have a 2-pointer approach but I'm getting stuck handling duplicate triplets..."
                  rows={2}
                  className="w-full p-3 rounded-xl bg-[#0a0c12] border border-white/[0.08] text-white placeholder-neutral-500 focus:outline-none focus:border-amber-400/60 text-xs transition-colors"
                />
              </div>

              {/* Action Button */}
              <div className="flex items-center justify-between pt-2">
                <span className="text-xs text-neutral-400 font-mono">
                  Current Balance: <strong className="text-amber-400">{credits}</strong> credits
                </span>

                <button
                  type="button"
                  disabled={hintLoading}
                  onClick={() => handleRequestHint(selectedHintLevel)}
                  className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-gradient-to-r from-amber-400 to-amber-500 text-black font-bold text-xs transition-all shadow-[0_0_15px_rgba(245,158,11,0.25)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)] disabled:opacity-50"
                >
                  <Sparkles className="h-4 w-4 stroke-[2.5]" />
                  {hintLoading
                    ? "Consulting Gemini AI..."
                    : `Unlock Tier ${selectedHintLevel} Hint (${hintLadderOptions[selectedHintLevel - 1].cost} cr)`}
                </button>
              </div>

              {hintError && (
                <div className="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-200 text-xs">
                  {hintError}
                </div>
              )}

              {/* Rendered Hint Result */}
              {hintResult && (
                <div className="p-5 rounded-2xl bg-[#0a0c12]/90 border border-amber-500/30 shadow-[0_0_20px_rgba(245,158,11,0.12)] space-y-3">
                  <div className="flex items-center justify-between text-xs font-mono">
                    <span className="inline-flex items-center gap-1.5 text-amber-400 font-bold">
                      <Sparkles className="h-3.5 w-3.5" /> Tier {hintResult.level} Hint Unlocked
                    </span>
                    <span className="text-[10px] text-neutral-400">
                      Model: {hintResult.model_used} &bull; -{hintResult.credits_deducted} cr
                    </span>
                  </div>
                  <div className="p-4 rounded-xl bg-black/40 border border-white/[0.06] text-xs text-neutral-200 leading-relaxed whitespace-pre-wrap font-sans">
                    {hintResult.hint_text}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Tab 2: Approach & Code Review */}
          {activeTab === "review" && (
            <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)] space-y-6">
              <div>
                <h2 className="text-base font-bold text-white flex items-center gap-2">
                  <Code2 className="h-5 w-5 text-amber-400" />
                  AI Approach &amp; Code Review
                </h2>
                <p className="text-xs text-neutral-400 mt-1">
                  Submit your code snippet and proposed complexities for structured critique
                  (Cost: {pricing.approach_review} credits).
                </p>
              </div>

              <form onSubmit={handleRequestReview} className="space-y-4">
                <div>
                  <label className="block text-xs font-medium text-neutral-300 mb-1.5 uppercase font-mono">
                    Candidate Code Snippet
                  </label>
                  <textarea
                    required
                    value={candidateCode}
                    onChange={(e) => setCandidateCode(e.target.value)}
                    placeholder="class Solution:&#10;    def twoSum(self, nums, target):&#10;        ..."
                    rows={8}
                    className="w-full p-3 rounded-xl bg-[#0a0c12] border border-white/[0.08] text-white placeholder-neutral-500 focus:outline-none focus:border-amber-400/60 font-mono text-xs transition-colors"
                  />
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs font-medium text-neutral-300 mb-1.5 uppercase font-mono">
                      Proposed Time Complexity
                    </label>
                    <input
                      type="text"
                      value={proposedTime}
                      onChange={(e) => setProposedTime(e.target.value)}
                      placeholder="O(N)"
                      className="w-full px-3 py-2 rounded-xl bg-[#0a0c12] border border-white/[0.08] text-white font-mono text-xs focus:outline-none focus:border-amber-400/60"
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-neutral-300 mb-1.5 uppercase font-mono">
                      Proposed Space Complexity
                    </label>
                    <input
                      type="text"
                      value={proposedSpace}
                      onChange={(e) => setProposedSpace(e.target.value)}
                      placeholder="O(1)"
                      className="w-full px-3 py-2 rounded-xl bg-[#0a0c12] border border-white/[0.08] text-white font-mono text-xs focus:outline-none focus:border-amber-400/60"
                    />
                  </div>
                </div>

                <div className="flex items-center justify-between pt-2">
                  <span className="text-xs text-neutral-400 font-mono">
                    Balance: <strong className="text-amber-400">{credits}</strong> credits
                  </span>
                  <button
                    type="submit"
                    disabled={reviewLoading}
                    className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-gradient-to-r from-amber-400 to-amber-500 text-black font-bold text-xs transition-all shadow-[0_0_15px_rgba(245,158,11,0.25)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)] disabled:opacity-50"
                  >
                    <Send className="h-3.5 w-3.5 stroke-[2.5]" />
                    {reviewLoading ? "Analyzing Approach..." : `Submit for Review (${pricing.approach_review} cr)`}
                  </button>
                </div>
              </form>

              {reviewError && (
                <div className="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-200 text-xs">
                  {reviewError}
                </div>
              )}

              {reviewResult && (
                <div className="p-5 rounded-2xl bg-[#0a0c12]/90 border border-amber-500/30 shadow-[0_0_20px_rgba(245,158,11,0.12)] space-y-3">
                  <div className="flex items-center justify-between text-xs font-mono">
                    <span className="inline-flex items-center gap-1.5 text-amber-400 font-bold">
                      <Code2 className="h-3.5 w-3.5" /> Code Review Report
                    </span>
                    <span className="text-[10px] text-neutral-400">
                      Model: {reviewResult.model_used} &bull; -{reviewResult.credits_deducted} cr
                    </span>
                  </div>
                  <div className="p-4 rounded-xl bg-black/40 border border-white/[0.06] text-xs text-neutral-200 leading-relaxed whitespace-pre-wrap font-sans">
                    {reviewResult.review_text}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Tab 3: Concept Q&A */}
          {activeTab === "explain" && (
            <div className="p-6 rounded-2xl bg-[#0e1017]/75 backdrop-blur-2xl border border-white/[0.08] shadow-[inset_0_1px_1px_rgba(255,255,255,0.08)] space-y-6">
              <div>
                <h2 className="text-base font-bold text-white flex items-center gap-2">
                  <HelpCircle className="h-5 w-5 text-amber-400" />
                  Algorithmic Concept Q&amp;A
                </h2>
                <p className="text-xs text-neutral-400 mt-1">
                  Ask deep theoretical questions regarding the core pattern or invariant used in this
                  problem (Cost: {pricing.concept_explain} credits).
                </p>
              </div>

              <form onSubmit={handleRequestExplain} className="space-y-4">
                <div>
                  <label className="block text-xs font-medium text-neutral-300 mb-1.5 uppercase font-mono">
                    Your Question
                  </label>
                  <textarea
                    required
                    value={explainQuery}
                    onChange={(e) => setExplainQuery(e.target.value)}
                    placeholder="e.g. Why does a monotonic deque guarantee O(N) sliding window maximum instead of a heap?"
                    rows={3}
                    className="w-full p-3 rounded-xl bg-[#0a0c12] border border-white/[0.08] text-white placeholder-neutral-500 focus:outline-none focus:border-amber-400/60 text-xs transition-colors"
                  />
                </div>

                <div className="flex items-center justify-between pt-2">
                  <span className="text-xs text-neutral-400 font-mono">
                    Balance: <strong className="text-amber-400">{credits}</strong> credits
                  </span>
                  <button
                    type="submit"
                    disabled={explainLoading}
                    className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-gradient-to-r from-amber-400 to-amber-500 text-black font-bold text-xs transition-all shadow-[0_0_15px_rgba(245,158,11,0.25)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)] disabled:opacity-50"
                  >
                    <Zap className="h-3.5 w-3.5 stroke-[2.5]" />
                    {explainLoading ? "Generating Guide..." : `Ask Question (${pricing.concept_explain} cr)`}
                  </button>
                </div>
              </form>

              {explainError && (
                <div className="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-200 text-xs">
                  {explainError}
                </div>
              )}

              {explainResult && (
                <div className="p-5 rounded-2xl bg-[#0a0c12]/90 border border-amber-500/30 shadow-[0_0_20px_rgba(245,158,11,0.12)] space-y-3">
                  <div className="flex items-center justify-between text-xs font-mono">
                    <span className="inline-flex items-center gap-1.5 text-amber-400 font-bold">
                      <HelpCircle className="h-3.5 w-3.5" /> Concept Explanation
                    </span>
                    <span className="text-[10px] text-neutral-400">
                      Model: {explainResult.model_used} &bull; -{explainResult.credits_deducted} cr
                    </span>
                  </div>
                  <div className="p-4 rounded-xl bg-black/40 border border-white/[0.06] text-xs text-neutral-200 leading-relaxed whitespace-pre-wrap font-sans">
                    {explainResult.explanation}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Record Attempt Modal */}
      {showAttemptModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/85 backdrop-blur-md animate-fade-in">
          <div className="bg-[#0a0d14]/95 border border-white/[0.12] rounded-3xl p-6 sm:p-8 max-w-lg w-full max-h-[90vh] overflow-y-auto shadow-[inset_0_1px_1px_rgba(255,255,255,0.15),0_24px_64px_rgba(0,0,0,0.7)] space-y-6">
            <div className="flex items-center justify-between border-b border-white/[0.08] pb-4">
              <div>
                <h3 className="text-lg font-bold text-white flex items-center gap-2">
                  <CheckCircle2 className="h-5 w-5 text-amber-400" />
                  Log Problem Attempt
                </h3>
                <p className="text-xs text-neutral-400 mt-0.5">
                  Feeds the SM-2 spaced repetition and canonical mastery engine.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setShowAttemptModal(false)}
                className="p-1 rounded-lg text-neutral-400 hover:text-white"
              >
                &times;
              </button>
            </div>

            {attemptFeedback ? (
              <div className="p-6 rounded-2xl bg-[#10131e]/80 border border-amber-500/30 text-center space-y-4">
                <CheckCircle2 className="h-10 w-10 text-amber-400 mx-auto" />
                <h4 className="text-base font-bold text-white">Attempt Successfully Recorded!</h4>
                <div className="p-4 rounded-xl bg-[#0a0c12] border border-white/[0.06] text-xs font-mono text-neutral-300 space-y-1.5 text-left">
                  <p>
                    <strong className="text-amber-400">Next Review: </strong>
                    {new Date(attemptFeedback.next_review_at).toLocaleDateString()} (in{" "}
                    {attemptFeedback.interval_days} days)
                  </p>
                  <p>
                    <strong className="text-amber-400">Ease Factor: </strong>
                    {attemptFeedback.ease_factor.toFixed(2)}
                  </p>
                  <p>
                    <strong className="text-amber-400">Repetitions: </strong>
                    {attemptFeedback.repetition_count}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => {
                    setShowAttemptModal(false);
                    setAttemptFeedback(null);
                  }}
                  className="w-full py-2.5 rounded-xl bg-gradient-to-r from-amber-400 to-amber-500 text-black font-bold text-xs shadow-[0_0_15px_rgba(245,158,11,0.25)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)]"
                >
                  Done
                </button>
              </div>
            ) : (
              <form onSubmit={handleRecordAttempt} className="space-y-4 text-xs text-white">
                <div>
                  <label className="block text-xs font-medium text-neutral-300 mb-1 uppercase font-mono">
                    Attempt Outcome
                  </label>
                  <select
                    value={outcome}
                    onChange={(e) => setOutcome(e.target.value)}
                    className="w-full p-2.5 rounded-xl bg-[#0a0c12] border border-white/[0.08] text-white text-xs focus:outline-none focus:border-amber-400/60"
                  >
                    <option value="SOLVED_INDEPENDENTLY">Solved Independently (Full Credit)</option>
                    <option value="SOLVED_WITH_HINTS">Solved With Hints</option>
                    <option value="SOLVED_AFTER_EDITORIAL">Solved After Editorial</option>
                    <option value="UNRESOLVED_FAILED">Unresolved / Failed</option>
                  </select>
                </div>

                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs font-medium text-neutral-300 mb-1 uppercase font-mono">
                      Perceived Difficulty
                    </label>
                    <select
                      value={perceivedDifficulty}
                      onChange={(e) => setPerceivedDifficulty(e.target.value)}
                      className="w-full p-2.5 rounded-xl bg-[#0a0c12] border border-white/[0.08] text-white text-xs focus:outline-none focus:border-amber-400/60"
                    >
                      <option value="EASY">Easy</option>
                      <option value="JUST_RIGHT">Just Right</option>
                      <option value="CHALLENGING">Challenging</option>
                      <option value="OVERWHELMING">Overwhelming</option>
                    </select>
                  </div>

                  <div>
                    <label className="block text-xs font-medium text-neutral-300 mb-1 uppercase font-mono">
                      Quality Rating (1 to 5)
                    </label>
                    <div className="flex items-center gap-1 py-1.5">
                      {[1, 2, 3, 4, 5].map((star) => (
                        <button
                          key={star}
                          type="button"
                          onClick={() => setQualityRating(star)}
                          className="p-1 text-neutral-500 hover:text-amber-400"
                        >
                          <Star
                            className={`h-4 w-4 ${
                              star <= qualityRating
                                ? "text-amber-400 fill-amber-400"
                                : "text-white/20"
                            }`}
                          />
                        </button>
                      ))}
                    </div>
                  </div>
                </div>

                {/* Mistake Tags */}
                <div>
                  <label className="block text-xs font-medium text-neutral-300 mb-1.5 uppercase font-mono flex items-center gap-1.5">
                    <Tag className="h-3.5 w-3.5 text-amber-400" />
                    Mistake Taxonomy (Select any that apply)
                  </label>
                  <div className="flex flex-wrap gap-1.5">
                    {standardMistakeTags.map((tag) => {
                      const isSelected = selectedMistakeTags.includes(tag);
                      return (
                        <button
                          key={tag}
                          type="button"
                          onClick={() => toggleMistakeTag(tag)}
                          className={`px-2.5 py-1 rounded-lg text-[10px] font-mono transition-colors ${
                            isSelected
                              ? "bg-amber-500 text-black border border-amber-400 font-bold"
                              : "bg-[#0a0c12] text-neutral-400 border border-white/[0.08] hover:border-amber-400/50"
                          }`}
                        >
                          #{tag}
                        </button>
                      );
                    })}
                  </div>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-300 mb-1 uppercase font-mono">
                    Time Spent
                  </label>
                  <p className="text-xs text-neutral-400 font-mono">
                    {timerSeconds > 0
                      ? `${formatTimer(timerSeconds)} (from interview timer)`
                      : "Defaulting to 15 minutes"}
                  </p>
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-300 mb-1 uppercase font-mono">
                    Approach Summary &amp; Notes
                  </label>
                  <textarea
                    value={approachSummary}
                    onChange={(e) => setApproachSummary(e.target.value)}
                    placeholder="Key realization: keep running sum and check if prefix_sum - k exists in hash map."
                    rows={2}
                    className="w-full p-2.5 rounded-xl bg-[#0a0c12] border border-white/[0.08] text-white text-xs focus:outline-none focus:border-amber-400/60"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-neutral-300 mb-1 uppercase font-mono">
                    Solution Code Snippet (Optional)
                  </label>
                  <textarea
                    value={attemptCode}
                    onChange={(e) => setAttemptCode(e.target.value)}
                    placeholder="def solution(): ..."
                    rows={3}
                    className="w-full p-2.5 rounded-xl bg-[#0a0c12] border border-white/[0.08] text-white text-xs font-mono focus:outline-none focus:border-amber-400/60"
                  />
                </div>

                <div className="pt-2 flex items-center justify-end gap-3">
                  <button
                    type="button"
                    onClick={() => setShowAttemptModal(false)}
                    className="px-4 py-2 rounded-xl text-xs font-semibold text-neutral-400 hover:text-white"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={isLoggingAttempt}
                    className="px-5 py-2.5 rounded-xl bg-gradient-to-r from-amber-400 to-amber-500 text-black font-bold text-xs transition-all shadow-[0_0_15px_rgba(245,158,11,0.25)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)] disabled:opacity-50"
                  >
                    {isLoggingAttempt ? "Saving..." : "Record & Update Schedule"}
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export default function QuestionDetailPage() {
  return (
    <React.Suspense
      fallback={
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 w-full animate-pulse space-y-6 min-h-screen">
          <div className="h-6 bg-amber-500/10 rounded w-1/5" />
          <div className="h-40 bg-[#0e1017]/60 rounded-2xl border border-white/[0.08]" />
          <div className="h-96 bg-[#0e1017]/60 rounded-2xl border border-white/[0.08]" />
        </div>
      }
    >
      <QuestionDetailContent />
    </React.Suspense>
  );
}
