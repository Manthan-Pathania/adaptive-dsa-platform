"use client";

import React, { useState } from "react";
import { AttemptOutcome, PerceivedDifficulty } from "@/types";
import { X, CheckCircle2, RotateCw } from "lucide-react";

interface QuickMarkModalProps {
  isOpen: boolean;
  questionTitle: string;
  questionDifficulty: string;
  patternName?: string;
  onClose: () => void;
  onSubmit: (data: {
    outcome: AttemptOutcome;
    difficulty_experienced: PerceivedDifficulty;
    needs_revision: boolean;
    confidence_first_try: boolean;
    notes: string;
  }) => Promise<void>;
  isSubmitting: boolean;
}

export function QuickMarkModal({
  isOpen,
  questionTitle,
  questionDifficulty,
  patternName,
  onClose,
  onSubmit,
  isSubmitting,
}: QuickMarkModalProps) {
  const [outcome, setOutcome] = useState<AttemptOutcome>("SOLVED_INDEPENDENTLY");
  const [difficultyExp, setDifficultyExp] = useState<PerceivedDifficulty>("JUST_RIGHT");
  const [needsRevision, setNeedsRevision] = useState(false);
  const [confidenceFirstTry, setConfidenceFirstTry] = useState(true);
  const [notes, setNotes] = useState("");

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await onSubmit({
      outcome,
      difficulty_experienced: difficultyExp,
      needs_revision: needsRevision,
      confidence_first_try: confidenceFirstTry,
      notes,
    });
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg bg-[#0a0d14]/95 backdrop-blur-3xl border border-white/[0.12] rounded-2xl shadow-[inset_0_1px_1px_rgba(255,255,255,0.18),0_20px_60px_rgba(0,0,0,0.8),0_0_35px_rgba(245,158,11,0.12)] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Modal Header */}
        <div className="p-5 border-b border-white/[0.08] flex items-start justify-between bg-[#0e111a]/90">
          <div>
            <div className="flex items-center gap-2 mb-1.5">
              <span className="px-2 py-0.5 rounded text-[10px] font-mono uppercase bg-amber-500/15 text-amber-400 border border-amber-500/30 font-bold">
                {questionDifficulty}
              </span>
              {patternName && (
                <span className="text-xs font-mono text-neutral-400">
                  {patternName}
                </span>
              )}
            </div>
            <h2 className="text-base font-bold text-white leading-snug">
              {questionTitle}
            </h2>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-neutral-400 hover:text-white hover:bg-white/[0.05] transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Modal Form */}
        <form onSubmit={handleSubmit} className="p-6 space-y-5 text-xs text-white">
          {/* Outcome Selection */}
          <div className="space-y-2">
            <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
              Solving Outcome
            </label>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              <button
                type="button"
                onClick={() => setOutcome("SOLVED_INDEPENDENTLY")}
                className={`p-2.5 rounded-xl border text-left transition-all ${
                  outcome === "SOLVED_INDEPENDENTLY"
                    ? "bg-gradient-to-r from-amber-500 to-amber-600 text-black border-amber-500 font-bold shadow-[0_0_15px_rgba(245,158,11,0.3)]"
                    : "bg-[#121520] border-white/[0.08] text-neutral-300 hover:border-amber-400/40 hover:bg-[#181b28]"
                }`}
              >
                <div className="font-semibold text-xs">Optimal / Independent</div>
                <div
                  className={`text-[10px] ${
                    outcome === "SOLVED_INDEPENDENTLY" ? "text-black/80" : "text-neutral-500"
                  }`}
                >
                  No hints needed
                </div>
              </button>

              <button
                type="button"
                onClick={() => setOutcome("SOLVED_WITH_HINTS")}
                className={`p-2.5 rounded-xl border text-left transition-all ${
                  outcome === "SOLVED_WITH_HINTS"
                    ? "bg-gradient-to-r from-amber-500 to-amber-600 text-black border-amber-500 font-bold shadow-[0_0_15px_rgba(245,158,11,0.3)]"
                    : "bg-[#121520] border-white/[0.08] text-neutral-300 hover:border-amber-400/40 hover:bg-[#181b28]"
                }`}
              >
                <div className="font-semibold text-xs">With AI / Hints</div>
                <div
                  className={`text-[10px] ${
                    outcome === "SOLVED_WITH_HINTS" ? "text-black/80" : "text-neutral-500"
                  }`}
                >
                  Needed guidance
                </div>
              </button>

              <button
                type="button"
                onClick={() => setOutcome("SOLVED_AFTER_EDITORIAL")}
                className={`p-2.5 rounded-xl border text-left transition-all ${
                  outcome === "SOLVED_AFTER_EDITORIAL"
                    ? "bg-gradient-to-r from-amber-500 to-amber-600 text-black border-amber-500 font-bold shadow-[0_0_15px_rgba(245,158,11,0.3)]"
                    : "bg-[#121520] border-white/[0.08] text-neutral-300 hover:border-amber-400/40 hover:bg-[#181b28]"
                }`}
              >
                <div className="font-semibold text-xs">Saw Solution</div>
                <div
                  className={`text-[10px] ${
                    outcome === "SOLVED_AFTER_EDITORIAL" ? "text-black/80" : "text-neutral-500"
                  }`}
                >
                  Need revision soon
                </div>
              </button>
            </div>
          </div>

          {/* Perceived Difficulty Selection */}
          <div className="space-y-2">
            <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
              Perceived Difficulty
            </label>
            <div className="grid grid-cols-3 gap-2">
              {(
                [
                  { val: "EASY" as const, label: "Easy / Smooth" },
                  { val: "JUST_RIGHT" as const, label: "Balanced" },
                  { val: "CHALLENGING" as const, label: "Challenging" },
                ]
              ).map((d) => (
                <button
                  key={d.val}
                  type="button"
                  onClick={() => setDifficultyExp(d.val)}
                  className={`py-2 px-3 rounded-xl border text-center transition-all ${
                    difficultyExp === d.val
                      ? "bg-gradient-to-r from-amber-500 to-amber-600 text-black border-amber-500 font-bold shadow-[0_0_12px_rgba(245,158,11,0.25)]"
                      : "bg-[#121520] border-white/[0.08] text-neutral-300 hover:border-amber-400/40"
                  }`}
                >
                  {d.label}
                </button>
              ))}
            </div>
          </div>

          {/* Toggles: SM-2 Revision & First-Try Confidence */}
          <div className="space-y-2.5 pt-1">
            <label className="flex items-center gap-3 p-3 rounded-xl bg-[#121520] border border-white/[0.08] cursor-pointer hover:border-amber-400/40 transition-colors shadow-[inset_0_1px_1px_rgba(255,255,255,0.06)]">
              <input
                type="checkbox"
                checked={needsRevision}
                onChange={(e) => setNeedsRevision(e.target.checked)}
                className="h-4 w-4 rounded accent-amber-500"
              />
              <div className="flex items-center gap-2">
                <RotateCw className="h-4 w-4 text-amber-400" />
                <span className="font-medium text-white">
                  Add to Spaced Revision (SM-2 Schedule)
                </span>
              </div>
            </label>

            <label className="flex items-center gap-3 p-3 rounded-xl bg-[#121520] border border-white/[0.08] cursor-pointer hover:border-amber-400/40 transition-colors shadow-[inset_0_1px_1px_rgba(255,255,255,0.06)]">
              <input
                type="checkbox"
                checked={confidenceFirstTry}
                onChange={(e) => setConfidenceFirstTry(e.target.checked)}
                className="h-4 w-4 rounded accent-amber-500"
              />
              <div className="flex items-center gap-2">
                <CheckCircle2 className="h-4 w-4 text-amber-400" />
                <span className="font-medium text-white">
                  Fully Confident on First Try (Mastery Counter)
                </span>
              </div>
            </label>
          </div>

          {/* Quick Notes */}
          <div className="space-y-1.5">
            <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
              Quick Reflection Notes (Optional)
            </label>
            <textarea
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="e.g. Solved with two pointers. Key trick was remembering to decrement j while arr[j] > target."
              rows={2}
              className="w-full p-3 rounded-xl bg-[#121520] border border-white/[0.1] focus:border-amber-400/50 text-xs text-white placeholder-neutral-500 focus:outline-none transition-colors resize-none shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
            />
          </div>

          {/* Action Buttons */}
          <div className="flex items-center justify-end gap-3 pt-3 border-t border-white/[0.08]">
            <button
              type="button"
              onClick={onClose}
              disabled={isSubmitting}
              className="px-4 py-2 rounded-xl text-neutral-400 hover:text-white border border-white/[0.08] hover:bg-white/[0.04] transition-colors font-medium text-xs"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={isSubmitting}
              className="px-5 py-2 rounded-xl text-xs font-bold bg-gradient-to-r from-amber-500 to-amber-600 text-black shadow-[0_0_20px_rgba(245,158,11,0.28)] hover:shadow-[0_0_30px_rgba(245,158,11,0.45)] transition-all flex items-center gap-2 disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <div className="h-3 w-3 border-2 border-black border-t-transparent rounded-full animate-spin" />
                  Saving...
                </>
              ) : (
                "Save & Mark Solved"
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
