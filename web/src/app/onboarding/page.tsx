"use client";

import React, { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { api } from "@/lib/api";
import {
  Building2,
  Calendar,
  Clock,
  Sparkles,
  ArrowRight,
  ArrowLeft,
  Check,
  Target,
  Brain,
  RotateCw,
} from "lucide-react";

const TARGET_COMPANIES = [
  { id: "Google", name: "Google", desc: "Algorithmic depth & problem solving" },
  { id: "Meta", name: "Meta (Facebook)", desc: "High execution speed & optimal code" },
  { id: "Amazon", name: "Amazon", desc: "Core patterns & Leadership Principles" },
  { id: "Microsoft", name: "Microsoft", desc: "Data structures & edge case handling" },
  { id: "Apple", name: "Apple", desc: "System efficiency & solid fundamentals" },
  { id: "Uber", name: "Uber", desc: "Concurrency, graphs & real-time scaling" },
  { id: "Netflix", name: "Netflix", desc: "System mastery & high-ownership DSA" },
  { id: "Bloomberg", name: "Bloomberg", desc: "Fast indexing, heaps & strings" },
];

const TIMELINES = [
  { id: "1_month", days: 30, label: "30 Days Intensive", sub: "Sprint (~2-3 hrs/day)" },
  { id: "2_months", days: 60, label: "60 Days Balanced", sub: "Standard FAANG prep (~1-2 hrs/day)", default: true },
  { id: "3_months", days: 90, label: "90 Days Deep Mastery", sub: "Comprehensive coverage (~1 hr/day)" },
];

const STUDY_TIMES = [
  { value: 60, label: "1 hour / day", desc: "Recommended balance" },
  { value: 90, label: "1.5 hours / day", desc: "Accelerated track" },
  { value: 120, label: "2 hours / day", desc: "Full-time interview prep" },
  { value: 180, label: "3+ hours / day", desc: "High intensity sprint" },
];

const EXPERIENCE_LEVELS = [
  { id: "beginner", title: "Beginner / CS Student", desc: "Refreshing core data structures and standard loops" },
  { id: "intermediate", title: "Intermediate Engineer", desc: "Familiar with trees and algorithms, seeking pattern mastery" },
  { id: "advanced", title: "Senior / Competitive", desc: "Strong algorithm fundamentals, targeting senior FAANG rounds" },
];

const DSA_PATTERNS = [
  "Arrays & Hashing",
  "Two Pointers",
  "Sliding Window",
  "Stack & Monotonic Stack",
  "Binary Search",
  "Linked List",
  "Trees & BST",
  "Heap / Priority Queue",
  "Backtracking",
  "Graphs (BFS/DFS)",
  "Dynamic Programming (1D & 2D)",
  "Greedy Algorithms",
];

export default function OnboardingPage() {
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();

  const [step, setStep] = useState(1);
  const [targetCompanies, setTargetCompanies] = useState<string[]>(["Google", "Meta"]);
  const [timelineId, setTimelineId] = useState("2_months");
  const [dailyMins, setDailyMins] = useState(120);
  const [experienceLevel, setExperienceLevel] = useState("intermediate");
  const [familiarPatterns, setFamiliarPatterns] = useState<string[]>(["Arrays & Hashing", "Two Pointers"]);
  const [focusNotes, setFocusNotes] = useState("");

  const [isSubmitting, setIsSubmitting] = useState(false);
  const [aiGeneratingStatus, setAiGeneratingStatus] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!authLoading && !user) {
      router.push("/login");
    }
  }, [user, authLoading, router]);

  const toggleCompany = (id: string) => {
    setTargetCompanies((prev) =>
      prev.includes(id) ? prev.filter((c) => c !== id) : [...prev, id]
    );
  };

  const togglePattern = (pat: string) => {
    setFamiliarPatterns((prev) =>
      prev.includes(pat) ? prev.filter((p) => p !== pat) : [...prev, pat]
    );
  };

  const handleNext = () => {
    setError(null);
    if (step === 1 && targetCompanies.length === 0) {
      setError("Please select at least one target company to tailor your roadmap.");
      return;
    }
    setStep((s) => Math.min(s + 1, 5));
  };

  const handleBack = () => {
    setError(null);
    setStep((s) => Math.max(s - 1, 1));
  };

  const handleFinish = async () => {
    setIsSubmitting(true);
    setError(null);
    setAiGeneratingStatus("Connecting to Google Gemini AI...");
    try {
      const selectedTimeline = TIMELINES.find((t) => t.id === timelineId);
      const days = selectedTimeline?.days || 60;

      // 1. Generate custom roadmap with Gemini API
      setAiGeneratingStatus(`Gemini 2.5 is synthesizing your ${days}-day roadmap for ${targetCompanies.join(", ")}...`);
      let createdRoadmapId: string | undefined = undefined;

      try {
        const createdRoadmap = await api.generateRoadmap({
          target_companies: targetCompanies,
          timeline_days: days,
          daily_hours: dailyMins / 60,
          experience_level: experienceLevel,
          familiar_patterns: familiarPatterns,
          focus_notes: focusNotes,
        });
        createdRoadmapId = createdRoadmap?.id;
      } catch (genErr) {
        console.warn("AI generation failed, fallback to canonical master roadmap:", genErr);
      }

      // 2. Save candidate profile
      setAiGeneratingStatus("Finalizing candidate profile and credits...");
      await api.updateProfile({
        target_companies: targetCompanies,
        timeline_label: timelineId,
        daily_study_mins: dailyMins,
        experience_level: experienceLevel,
        familiar_patterns: familiarPatterns,
        is_onboarding_completed: true,
        selected_roadmap_id: createdRoadmapId,
      });

      router.push("/dashboard");
    } catch (err: unknown) {
      const e = err as Error;
      setError(e.message || "Failed to finalize roadmap. Please retry.");
      setIsSubmitting(false);
      setAiGeneratingStatus(null);
    }
  };

  if (authLoading) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="h-8 w-8 animate-spin rounded-full border-2 border-amber-400 border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="min-h-screen py-10 px-4 sm:px-6 lg:px-8 max-w-4xl mx-auto text-white flex flex-col justify-center">
      {/* Top Banner & Progress Header */}
      <div className="text-center mb-8">
        <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-amber-500/15 border border-amber-500/30 text-amber-300 text-xs font-mono font-bold mb-3">
          <Sparkles className="h-3.5 w-3.5" />
          <span>Step {step} of 5 • Tailored FAANG Curriculum</span>
        </div>
        <h1 className="text-3xl font-extrabold text-white tracking-tight">
          Welcome{user?.full_name ? `, ${user.full_name}` : ""}! Design Your AI Roadmap
        </h1>
        <p className="text-neutral-400 text-xs sm:text-sm mt-1.5 max-w-xl mx-auto">
          Gemini will synthesize a custom 3-stage roadmap using verified problems from the Master Question Bank.
        </p>

        {/* Progress Bar */}
        <div className="w-full max-w-md mx-auto mt-6 bg-[#0a0c12] border border-white/[0.08] rounded-full h-2 overflow-hidden p-0.5">
          <div
            className="bg-gradient-to-r from-amber-400 to-amber-500 h-full rounded-full transition-all duration-300 shadow-[0_0_10px_rgba(245,158,11,0.5)]"
            style={{ width: `${(step / 5) * 100}%` }}
          />
        </div>
      </div>

      {error && (
        <div className="mb-6 p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-200 text-xs">
          {error}
        </div>
      )}

      {/* Main Form Card */}
      <div className="bg-[#0e1017]/75 border border-white/[0.08] rounded-3xl p-6 sm:p-10 backdrop-blur-2xl shadow-[inset_0_1px_1px_rgba(255,255,255,0.12),0_20px_50px_rgba(0,0,0,0.5)]">
        {/* STEP 1: Target Companies */}
        {step === 1 && (
          <div className="space-y-6">
            <div className="flex items-center gap-3">
              <div className="p-3 rounded-2xl bg-amber-500/15 text-amber-400 border border-amber-500/30">
                <Building2 className="h-6 w-6 stroke-[2.2]" />
              </div>
              <div>
                <h2 className="text-xl font-bold text-white">Target Companies</h2>
                <p className="text-xs text-neutral-400">
                  Select the companies you are interviewing with to prioritize high-frequency problem sets.
                </p>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
              {TARGET_COMPANIES.map((company) => {
                const selected = targetCompanies.includes(company.id);
                return (
                  <button
                    key={company.id}
                    type="button"
                    onClick={() => toggleCompany(company.id)}
                    className={`flex items-start gap-3 p-4 rounded-2xl border text-left transition-all ${
                      selected
                        ? "bg-gradient-to-r from-amber-400 to-amber-500 text-black border-amber-400 font-bold shadow-[0_0_15px_rgba(245,158,11,0.25)]"
                        : "bg-[#0a0c12]/60 border-white/[0.08] text-white hover:border-amber-400/50 hover:bg-[#121520]/80"
                    }`}
                  >
                    <div
                      className={`h-5 w-5 mt-0.5 rounded-md flex items-center justify-center shrink-0 transition-colors ${
                        selected ? "bg-black text-amber-400" : "border border-white/20 bg-black/40"
                      }`}
                    >
                      {selected && <Check className="h-3.5 w-3.5 stroke-[3]" />}
                    </div>
                    <div>
                      <div className="text-sm font-semibold">{company.name}</div>
                      <div className={`text-xs mt-0.5 ${selected ? "text-black/80 font-medium" : "text-neutral-400"}`}>
                        {company.desc}
                      </div>
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {/* STEP 2: Timeline */}
        {step === 2 && (
          <div className="space-y-6">
            <div className="flex items-center gap-3">
              <div className="p-3 rounded-2xl bg-amber-500/15 text-amber-400 border border-amber-500/30">
                <Calendar className="h-6 w-6 stroke-[2.2]" />
              </div>
              <div>
                <h2 className="text-xl font-bold text-white">Interview Timeline</h2>
                <p className="text-xs text-neutral-400">
                  How many days do you have until your scheduled interview loop?
                </p>
              </div>
            </div>

            <div className="grid grid-cols-1 gap-3 pt-2">
              {TIMELINES.map((t) => {
                const selected = timelineId === t.id;
                return (
                  <button
                    key={t.id}
                    type="button"
                    onClick={() => setTimelineId(t.id)}
                    className={`flex items-center justify-between p-4 rounded-2xl border text-left transition-all ${
                      selected
                        ? "bg-gradient-to-r from-amber-400 to-amber-500 text-black border-amber-400 font-bold shadow-[0_0_15px_rgba(245,158,11,0.25)]"
                        : "bg-[#0a0c12]/60 border-white/[0.08] text-white hover:border-amber-400/50 hover:bg-[#121520]/80"
                    }`}
                  >
                    <div>
                      <div className="text-sm font-semibold">{t.label}</div>
                      <div className={`text-xs mt-0.5 ${selected ? "text-black/80 font-medium" : "text-neutral-400"}`}>
                        {t.sub}
                      </div>
                    </div>
                    <div
                      className={`h-5 w-5 rounded-full flex items-center justify-center ${
                        selected ? "bg-black text-amber-400" : "border border-white/20"
                      }`}
                    >
                      {selected && <Check className="h-3.5 w-3.5 stroke-[3]" />}
                    </div>
                  </button>
                );
              })}
            </div>
          </div>
        )}

        {/* STEP 3: Study Time & Experience */}
        {step === 3 && (
          <div className="space-y-6">
            <div className="flex items-center gap-3">
              <div className="p-3 rounded-2xl bg-amber-500/15 text-amber-400 border border-amber-500/30">
                <Clock className="h-6 w-6 stroke-[2.2]" />
              </div>
              <div>
                <h2 className="text-xl font-bold text-white">Daily Study Availability</h2>
                <p className="text-xs text-neutral-400">
                  How much dedicated focus time can you allocate per day?
                </p>
              </div>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 pt-2">
              {STUDY_TIMES.map((st) => {
                const selected = dailyMins === st.value;
                return (
                  <button
                    key={st.value}
                    type="button"
                    onClick={() => setDailyMins(st.value)}
                    className={`p-4 rounded-2xl border text-left transition-all ${
                      selected
                        ? "bg-gradient-to-r from-amber-400 to-amber-500 text-black border-amber-400 font-bold shadow-[0_0_15px_rgba(245,158,11,0.25)]"
                        : "bg-[#0a0c12]/60 border-white/[0.08] text-white hover:border-amber-400/50 hover:bg-[#121520]/80"
                    }`}
                  >
                    <div className="text-sm font-semibold">{st.label}</div>
                    <div className={`text-xs mt-0.5 ${selected ? "text-black/80" : "text-neutral-400"}`}>
                      {st.desc}
                    </div>
                  </button>
                );
              })}
            </div>

            <div className="space-y-2 pt-2">
              <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block">
                Experience Level
              </label>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
                {EXPERIENCE_LEVELS.map((lvl) => {
                  const selected = experienceLevel === lvl.id;
                  return (
                    <button
                      key={lvl.id}
                      type="button"
                      onClick={() => setExperienceLevel(lvl.id)}
                      className={`p-3 rounded-xl border text-left transition-all ${
                        selected
                          ? "bg-gradient-to-r from-amber-400 to-amber-500 text-black border-amber-400 font-bold"
                          : "bg-[#0a0c12]/60 border-white/[0.08] text-white hover:border-amber-400/50"
                      }`}
                    >
                      <div className="text-xs font-semibold">{lvl.title}</div>
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
        )}

        {/* STEP 4: Familiar Patterns & Weaknesses */}
        {step === 4 && (
          <div className="space-y-6">
            <div className="flex items-center gap-3">
              <div className="p-3 rounded-2xl bg-amber-500/15 text-amber-400 border border-amber-500/30">
                <Brain className="h-6 w-6 stroke-[2.2]" />
              </div>
              <div>
                <h2 className="text-xl font-bold text-white">Pattern Familiarity</h2>
                <p className="text-xs text-neutral-400">
                  Select patterns you already feel comfortable with. Gemini will adjust your early stages.
                </p>
              </div>
            </div>

            <div className="flex flex-wrap gap-2 pt-2">
              {DSA_PATTERNS.map((pat) => {
                const selected = familiarPatterns.includes(pat);
                return (
                  <button
                    key={pat}
                    type="button"
                    onClick={() => togglePattern(pat)}
                    className={`px-3 py-1.5 rounded-xl border text-xs font-mono transition-all ${
                      selected
                        ? "bg-gradient-to-r from-amber-400 to-amber-500 text-black border-amber-400 font-bold shadow-[0_0_10px_rgba(245,158,11,0.25)]"
                        : "bg-[#0a0c12]/60 border-white/[0.08] text-neutral-300 hover:border-amber-400/50"
                    }`}
                  >
                    {selected && "✓ "}
                    {pat}
                  </button>
                );
              })}
            </div>

            <div className="space-y-1.5 pt-2">
              <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block">
                Specific Focus Areas or Weaknesses (Optional)
              </label>
              <input
                type="text"
                value={focusNotes}
                onChange={(e) => setFocusNotes(e.target.value)}
                placeholder="e.g. Need more practice on Dynamic Programming and Binary Trees..."
                className="w-full p-2.5 rounded-xl bg-[#0a0c12] border border-white/[0.08] text-white placeholder-neutral-500 text-xs focus:outline-none focus:border-amber-400/60"
              />
            </div>
          </div>
        )}

        {/* STEP 5: Confirmation & Gemini AI Synthesis */}
        {step === 5 && (
          <div className="space-y-6">
            <div className="flex items-center gap-3">
              <div className="p-3 rounded-2xl bg-amber-500/15 text-amber-400 border border-amber-500/30">
                <Target className="h-6 w-6 stroke-[2.2]" />
              </div>
              <div>
                <h2 className="text-xl font-bold text-white">Review & Confirm Parameters</h2>
                <p className="text-xs text-neutral-400">
                  Gemini AI will synthesize a custom 3-stage roadmap using verified problems from the Master Question Bank.
                </p>
              </div>
            </div>

            <div className="p-5 rounded-2xl bg-[#0a0c12] border border-white/[0.08] space-y-3 font-mono text-xs">
              <div className="flex justify-between border-b border-white/[0.06] pb-2">
                <span className="text-neutral-400">Target Companies:</span>
                <span className="text-amber-400 font-bold">{targetCompanies.join(", ")}</span>
              </div>
              <div className="flex justify-between border-b border-white/[0.06] pb-2">
                <span className="text-neutral-400">Timeline:</span>
                <span className="text-white font-bold">
                  {TIMELINES.find((t) => t.id === timelineId)?.label}
                </span>
              </div>
              <div className="flex justify-between border-b border-white/[0.06] pb-2">
                <span className="text-neutral-400">Daily Commitment:</span>
                <span className="text-white font-bold">{dailyMins} minutes / day</span>
              </div>
              <div className="flex justify-between border-b border-white/[0.06] pb-2">
                <span className="text-neutral-400">Experience:</span>
                <span className="text-amber-400 font-bold uppercase">{experienceLevel}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-neutral-400">Welcome AI Credits:</span>
                <span className="text-amber-400 font-bold">50 Credits Granted</span>
              </div>
            </div>

            {aiGeneratingStatus && (
              <div className="p-4 rounded-xl border border-amber-500/40 bg-amber-500/15 text-white flex items-center gap-3">
                <RotateCw className="h-5 w-5 text-amber-400 animate-spin shrink-0" />
                <span className="text-xs font-mono">{aiGeneratingStatus}</span>
              </div>
            )}
          </div>
        )}

        {/* Action Controls */}
        <div className="mt-8 pt-6 border-t border-white/[0.06] flex items-center justify-between">
          {step > 1 ? (
            <button
              type="button"
              onClick={handleBack}
              disabled={isSubmitting}
              className="inline-flex items-center gap-2 px-5 py-2.5 rounded-xl text-xs font-semibold text-neutral-400 hover:text-white border border-white/[0.1] hover:border-amber-400/50 transition-colors"
            >
              <ArrowLeft className="h-4 w-4" />
              <span>Back</span>
            </button>
          ) : (
            <div />
          )}

          {step < 5 ? (
            <button
              type="button"
              onClick={handleNext}
              className="inline-flex items-center gap-2 px-6 py-2.5 rounded-xl text-xs font-bold bg-gradient-to-r from-amber-400 to-amber-500 text-black shadow-[0_0_15px_rgba(245,158,11,0.25)] hover:shadow-[0_0_25px_rgba(245,158,11,0.4)] transition-all"
            >
              <span>Continue</span>
              <ArrowRight className="h-4 w-4" />
            </button>
          ) : (
            <button
              type="button"
              onClick={handleFinish}
              disabled={isSubmitting}
              className="inline-flex items-center gap-2 px-8 py-3 rounded-xl text-xs font-bold bg-gradient-to-r from-amber-400 to-amber-500 text-black shadow-[0_0_25px_rgba(245,158,11,0.35)] hover:shadow-[0_0_35px_rgba(245,158,11,0.5)] hover:scale-[1.02] transition-all disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <div className="h-4 w-4 border-2 border-black border-t-transparent animate-spin rounded-full" />
                  <span>Synthesizing with Gemini...</span>
                </>
              ) : (
                <>
                  <Sparkles className="h-4 w-4 text-black stroke-[2.5]" />
                  <span>Generate Roadmap & Start Preparation</span>
                </>
              )}
            </button>
          )}
        </div>
      </div>
    </div>
  );
}
