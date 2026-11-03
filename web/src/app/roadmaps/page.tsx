"use client";

import React, { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";
import { useAuth } from "@/lib/auth-context";
import { Roadmap, UserProfile } from "@/types";
import {
  Map,
  Sparkles,
  ArrowRight,
  X,
  Check,
  FileSpreadsheet,
  Upload,
  Download,
  Info,
  Calendar,
  Layers,
  ListOrdered,
  Trash2,
  AlertTriangle,
} from "lucide-react";

const POPULAR_COMPANIES = [
  "Google",
  "Meta",
  "Amazon",
  "Apple",
  "Microsoft",
  "Netflix",
  "Uber",
  "Bloomberg",
  "Stripe",
  "Palantir",
];

export default function RoadmapsPage() {
  const { user } = useAuth();
  const router = useRouter();
  const [roadmaps, setRoadmaps] = useState<Roadmap[]>([]);
  const [profile, setProfile] = useState<UserProfile | null>(null);
  const [loading, setLoading] = useState(true);
  const [activatingId, setActivatingId] = useState<string | null>(null);
  const [message, setMessage] = useState<string | null>(null);

  // AI Roadmap Modal state
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [selectedCompanies, setSelectedCompanies] = useState<string[]>(["Google", "Meta"]);
  const [timelineDays, setTimelineDays] = useState(60);
  const [dailyHours, setDailyHours] = useState(2);
  const [experienceLevel, setExperienceLevel] = useState("intermediate");
  const [focusNotes, setFocusNotes] = useState("");
  const [isGenerating, setIsGenerating] = useState(false);
  const [genError, setGenError] = useState<string | null>(null);

  // Excel Import Modal state
  const [isImportModalOpen, setIsImportModalOpen] = useState(false);
  const [importTitle, setImportTitle] = useState("");
  const [importDays, setImportDays] = useState(30);
  const [importDescription, setImportDescription] = useState("");
  const [importArrangeBy, setImportArrangeBy] = useState<"excel_order" | "pattern">("excel_order");
  const [importFile, setImportFile] = useState<File | null>(null);
  const [isImporting, setIsImporting] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const [isDownloadingTemplate, setIsDownloadingTemplate] = useState(false);

  // Delete Roadmap state
  const [roadmapToDelete, setRoadmapToDelete] = useState<Roadmap | null>(null);
  const [isDeletingRoadmap, setIsDeletingRoadmap] = useState(false);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  useEffect(() => {
    let isMounted = true;
    async function loadData() {
      try {
        const [rData, prof] = await Promise.all([
          api.listRoadmaps().catch(() => ({ roadmaps: [] })),
          api.getProfile().catch(() => null),
        ]);
        if (isMounted) {
          setRoadmaps(rData.roadmaps || []);
          setProfile(prof);
        }
      } finally {
        if (isMounted) setLoading(false);
      }
    }
    loadData();
    return () => {
      isMounted = false;
    };
  }, []);

  const handleSetActiveRoadmap = async (roadmapId: string) => {
    setActivatingId(roadmapId);
    setMessage(null);
    try {
      const updated = await api.updateProfile({
        selected_roadmap_id: roadmapId,
      });
      setProfile(updated);
      setMessage("Active preparation roadmap updated successfully!");
      setTimeout(() => setMessage(null), 3000);
    } catch (err: unknown) {
      const e = err as Error;
      setMessage(`Failed to update roadmap: ${e.message}`);
    } finally {
      setActivatingId(null);
    }
  };

  const handleGenerateRoadmap = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!user) {
      router.push("/login");
      return;
    }
    if (selectedCompanies.length === 0) {
      setGenError("Please select at least one target company.");
      return;
    }
    setIsGenerating(true);
    setGenError(null);
    try {
      const created = await api.generateRoadmap({
        target_companies: selectedCompanies,
        timeline_days: timelineDays,
        daily_hours: dailyHours,
        experience_level: experienceLevel,
        focus_notes: focusNotes,
      });

      setRoadmaps((prev) => [created, ...prev]);
      if (profile) {
        setProfile({ ...profile, selected_roadmap_id: created.id });
      }
      setIsModalOpen(false);
      setMessage(`AI Custom Roadmap "${created.title}" successfully created and activated!`);
      setTimeout(() => setMessage(null), 4000);
    } catch (err: unknown) {
      const e = err as Error;
      setGenError(e.message || "Failed to generate roadmap via Gemini.");
    } finally {
      setIsGenerating(false);
    }
  };

  const handleDownloadTemplate = async () => {
    try {
      setIsDownloadingTemplate(true);
      const blob = await api.downloadRoadmapTemplate();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "Roadmap_Import_Template.xlsx";
      document.body.appendChild(a);
      a.click();
      window.URL.revokeObjectURL(url);
      document.body.removeChild(a);
    } catch (err: unknown) {
      const e = err as Error;
      setImportError(e.message || "Failed to download roadmap template");
    } finally {
      setIsDownloadingTemplate(false);
    }
  };

  const handleImportRoadmap = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!user) {
      router.push("/login");
      return;
    }
    if (!importTitle.trim()) {
      setImportError("Please enter a name for your roadmap.");
      return;
    }
    if (!importFile) {
      setImportError("Please select an Excel file (.xlsx or .xls) to import.");
      return;
    }

    setIsImporting(true);
    setImportError(null);
    try {
      const fd = new FormData();
      fd.append("title", importTitle.trim());
      fd.append("target_days", importDays.toString());
      if (importDescription.trim()) {
        fd.append("description", importDescription.trim());
      }
      fd.append("arrange_by", importArrangeBy);
      fd.append("file", importFile);

      const created = await api.importRoadmap(fd);
      setRoadmaps((prev) => [created, ...prev]);
      setIsImportModalOpen(false);
      setMessage(`Roadmap "${created.title}" imported successfully!`);
      router.push(`/roadmaps/${created.slug}`);
    } catch (err: unknown) {
      const e = err as Error;
      setImportError(e.message || "Failed to import roadmap.");
    } finally {
      setIsImporting(false);
    }
  };

  const handleDeleteRoadmap = async () => {
    if (!roadmapToDelete) return;
    setIsDeletingRoadmap(true);
    setDeleteError(null);
    try {
      await api.deleteRoadmap(roadmapToDelete.id);
      setRoadmaps((prev) => prev.filter((r) => r.id !== roadmapToDelete.id));
      if (profile?.selected_roadmap_id === roadmapToDelete.id) {
        const sys = roadmaps.find((r) => r.is_system_template && r.id !== roadmapToDelete.id);
        if (sys) {
          await api.updateProfile({ selected_roadmap_id: sys.id }).catch(() => {});
          setProfile((prev) => prev ? { ...prev, selected_roadmap_id: sys.id } : null);
        }
      }
      setMessage(`Roadmap "${roadmapToDelete.title}" deleted successfully.`);
      setTimeout(() => setMessage(null), 3500);
      setRoadmapToDelete(null);
    } catch (err: unknown) {
      const e = err as Error;
      setDeleteError(e.message || "Failed to delete roadmap.");
    } finally {
      setIsDeletingRoadmap(false);
    }
  };

  const toggleCompany = (comp: string) => {
    if (selectedCompanies.includes(comp)) {
      setSelectedCompanies(selectedCompanies.filter((c) => c !== comp));
    } else {
      setSelectedCompanies([...selectedCompanies, comp]);
    }
  };

  if (loading) {
    return (
      <div className="p-6 md:p-10 max-w-7xl mx-auto w-full space-y-8 animate-pulse min-h-screen">
        <div className="h-10 bg-white/[0.04] rounded-xl w-1/3 border border-white/[0.06]" />
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          {[1, 2].map((i) => (
            <div key={i} className="h-64 bg-[#10131d]/60 rounded-2xl border border-white/[0.06]" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="p-6 md:p-10 max-w-7xl mx-auto w-full space-y-8 text-white min-h-screen">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2 mb-2">
            <span className="w-1 h-3.5 bg-amber-400 rounded-sm inline-block shadow-[0_0_8px_rgba(245,158,11,0.5)]" />
            <span className="text-[11px] font-semibold text-neutral-400 tracking-widest uppercase">
              PREPARATION TRACKS
            </span>
            <span className="text-xs font-mono text-amber-400/90 ml-2">
              Gemini AI & Excel Import
            </span>
          </div>
          <h1 className="text-2xl md:text-3xl font-extrabold tracking-tight text-white flex items-center gap-3">
            <div className="p-2 rounded-xl bg-amber-500/15 border border-amber-500/30 text-amber-400 shadow-sm">
              <Map className="h-6 w-6 stroke-[2.2]" />
            </div>
            Preparation Roadmaps
          </h1>
          <p className="text-xs md:text-sm text-neutral-400 mt-1 max-w-2xl">
            Choose your learning path: explore the canonical Master Curriculum, import a custom Excel plan with your own questions and ordering, or synthesize an AI-tailored roadmap.
          </p>
        </div>

        {/* Action Buttons */}
        <div className="flex flex-wrap items-center gap-3 self-start md:self-auto shrink-0">
          <button
            onClick={() => {
              setImportError(null);
              setImportFile(null);
              setImportTitle("");
              setImportDescription("");
              setIsImportModalOpen(true);
            }}
            className="flex items-center gap-2 px-4 py-2.5 rounded-xl text-xs font-bold bg-[#141824] hover:bg-[#1a1f30] text-amber-300 border border-amber-500/30 hover:border-amber-400/60 shadow-[0_4px_16px_rgba(0,0,0,0.4)] transition-all"
          >
            <FileSpreadsheet className="h-4 w-4 text-amber-400 stroke-[2.2]" />
            + Import Roadmap (.xlsx)
          </button>

          <button
            onClick={() => {
              setGenError(null);
              setIsModalOpen(true);
            }}
            className="flex items-center gap-2 px-4 py-2.5 rounded-xl text-xs font-bold bg-gradient-to-r from-amber-500 to-amber-600 text-black shadow-[0_0_20px_rgba(245,158,11,0.28)] hover:shadow-[0_0_30px_rgba(245,158,11,0.45)] transition-all"
          >
            <Sparkles className="h-4 w-4 text-black stroke-[2.4]" />
            + Create New Roadmap with AI
          </button>
        </div>
      </div>

      {message && (
        <div className="p-4 rounded-xl border border-amber-500/30 bg-amber-500/10 text-white text-xs flex items-center justify-between backdrop-blur-xl">
          <span>{message}</span>
          <button onClick={() => setMessage(null)} className="text-amber-400 font-bold">
            Dismiss
          </button>
        </div>
      )}

      {/* Roadmaps Grid with Frosted Glass */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
        {roadmaps.map((r) => {
          const isActive = profile?.selected_roadmap_id === r.id;
          const isSystem = r.is_system_template;

          return (
            <div
              key={r.id}
              className={`p-6 rounded-2xl backdrop-blur-2xl transition-all flex flex-col justify-between ${isActive
                  ? "bg-[#141724]/85 border border-amber-500/50 shadow-[inset_0_1px_1px_rgba(255,255,255,0.18),0_0_35px_rgba(245,158,11,0.12)]"
                  : "bg-[#0e111a]/70 border border-white/[0.09] hover:border-amber-400/40 hover:bg-[#131724]/80 shadow-[inset_0_1px_1px_rgba(255,255,255,0.10),0_12px_32px_rgba(0,0,0,0.45)]"
                }`}
            >
              <div className="space-y-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-2">
                    <span
                      className={`px-2.5 py-0.5 rounded text-[10px] font-mono font-bold uppercase tracking-wider ${isSystem
                          ? "bg-white/[0.05] border border-white/[0.1] text-neutral-300"
                          : "bg-gradient-to-r from-amber-500 to-amber-600 text-black shadow-sm"
                        }`}
                    >
                      {isSystem ? "CANONICAL CURRICULUM" : "CUSTOM ROADMAP"}
                    </span>
                    {r.target_days && (
                      <span className="text-[11px] font-mono text-amber-400 font-semibold">
                        {r.target_days} Days
                      </span>
                    )}
                  </div>
                  {isActive && (
                    <span className="px-2.5 py-0.5 rounded text-[10px] font-mono font-bold bg-amber-500/15 text-amber-400 border border-amber-500/40 flex items-center gap-1 shadow-sm">
                      <Check className="h-3 w-3 stroke-[3]" /> ACTIVE
                    </span>
                  )}
                </div>

                <h2 className="text-lg font-bold text-white leading-tight">
                  {r.title}
                </h2>
                <p className="text-xs text-neutral-400 leading-relaxed line-clamp-2">
                  {r.description || "Comprehensive pattern-based algorithmic preparation sequence."}
                </p>
              </div>

              <div className="pt-6 mt-4 border-t border-white/[0.06] flex items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  {!isActive && user && (
                    <button
                      onClick={() => handleSetActiveRoadmap(r.id)}
                      disabled={activatingId === r.id}
                      className="px-3.5 py-1.5 rounded-xl text-xs font-semibold text-neutral-300 hover:text-white border border-white/[0.1] bg-[#121520] hover:border-amber-400/50 transition-colors shadow-sm cursor-pointer"
                    >
                      {activatingId === r.id ? "Setting..." : "Set as Active"}
                    </button>
                  )}
                  {!isSystem && user && (
                    <button
                      onClick={() => {
                        setDeleteError(null);
                        setRoadmapToDelete(r);
                      }}
                      className="p-1.5 rounded-xl text-neutral-400 hover:text-rose-400 border border-white/[0.08] hover:border-rose-500/40 hover:bg-rose-500/10 transition-colors cursor-pointer"
                      title="Delete Roadmap"
                    >
                      <Trash2 className="h-4 w-4" />
                    </button>
                  )}
                </div>

                <Link
                  href={`/roadmaps/${r.slug}`}
                  className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-xs font-bold text-black bg-gradient-to-r from-amber-500 to-amber-600 hover:shadow-[0_0_20px_rgba(245,158,11,0.35)] transition-all cursor-pointer"
                >
                  Explore Roadmap <ArrowRight className="h-3.5 w-3.5" />
                </Link>
              </div>
            </div>
          );
        })}
      </div>

      {/* Excel Roadmap Import Modal */}
      {isImportModalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md"
          onClick={() => !isImporting && setIsImportModalOpen(false)}
        >
          <div
            className="w-full max-w-xl bg-[#0a0d14]/95 backdrop-blur-3xl border border-white/[0.12] rounded-2xl shadow-[inset_0_1px_1px_rgba(255,255,255,0.18),0_20px_60px_rgba(0,0,0,0.8),0_0_35px_rgba(245,158,11,0.12)] overflow-hidden max-h-[92vh] flex flex-col"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Modal Header */}
            <div className="p-5 border-b border-white/[0.08] flex items-center justify-between bg-[#0e111a]/90 shrink-0">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-amber-500/15 border border-amber-500/25 text-amber-400">
                  <FileSpreadsheet className="h-4 w-4 stroke-[2.4]" />
                </div>
                <div>
                  <h2 className="text-sm font-bold text-white">Import Custom Roadmap from Excel</h2>
                  <p className="text-[11px] text-neutral-400 font-mono">Upload .xlsx or .xls curriculum sheet</p>
                </div>
              </div>
              <button
                onClick={() => !isImporting && setIsImportModalOpen(false)}
                disabled={isImporting}
                className="p-1.5 rounded-lg text-neutral-400 hover:text-white hover:bg-white/[0.05]"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {/* Modal Body */}
            <form onSubmit={handleImportRoadmap} className="p-6 space-y-4 text-xs text-white overflow-y-auto">
              {importError && (
                <div className="p-3 rounded-xl border border-rose-500/40 bg-rose-500/10 text-rose-200 text-[11px] leading-relaxed">
                  {importError}
                </div>
              )}

              {/* Roadmap Title */}
              <div className="space-y-1.5">
                <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
                  Roadmap Name <span className="text-amber-400">*</span>
                </label>
                <input
                  type="text"
                  required
                  value={importTitle}
                  onChange={(e) => setImportTitle(e.target.value)}
                  placeholder="e.g. TCS NQT Preparation Track, Striver SDE Sheet, NeetCode 150..."
                  className="w-full p-2.5 rounded-xl bg-[#121520] border border-white/[0.1] focus:border-amber-400/50 text-white placeholder-neutral-500 text-xs focus:outline-none transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
                />
              </div>

              {/* Timeline & Initial Arrange Preference */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold flex items-center gap-1.5">
                    <Calendar className="h-3 w-3 text-amber-400" /> Target Timeline (Days)
                  </label>
                  <input
                    type="number"
                    min={1}
                    max={365}
                    value={importDays}
                    onChange={(e) => setImportDays(parseInt(e.target.value) || 30)}
                    className="w-full p-2.5 rounded-xl bg-[#121520] border border-white/[0.1] focus:border-amber-400/50 text-white text-xs focus:outline-none transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
                    Default Arrangement
                  </label>
                  <select
                    value={importArrangeBy}
                    onChange={(e) => setImportArrangeBy(e.target.value as "excel_order" | "pattern")}
                    className="w-full p-2.5 rounded-xl bg-[#121520] border border-white/[0.1] focus:border-amber-400/50 text-white text-xs focus:outline-none transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
                  >
                    <option value="excel_order">Excel Sheet Order (Preserve sequence)</option>
                    <option value="pattern">Pattern-Wise (Group by Pattern)</option>
                  </select>
                </div>
              </div>

              {/* Description (Optional) */}
              <div className="space-y-1.5">
                <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
                  Description / Focus Notes (Optional)
                </label>
                <input
                  type="text"
                  value={importDescription}
                  onChange={(e) => setImportDescription(e.target.value)}
                  placeholder="e.g. Master sequence for TCS NQT 2024 coding section."
                  className="w-full p-2.5 rounded-xl bg-[#121520] border border-white/[0.1] focus:border-amber-400/50 text-white placeholder-neutral-500 text-xs focus:outline-none transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
                />
              </div>

              {/* Mandatory Format Specification Guide */}
              <div className="p-3.5 rounded-xl border border-amber-500/25 bg-amber-500/[0.04] space-y-2.5">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-1.5 text-amber-300 font-mono text-[11px] font-bold uppercase tracking-wider">
                    <Info className="h-3.5 w-3.5 text-amber-400" /> Required Excel Columns
                  </div>
                  <button
                    type="button"
                    onClick={handleDownloadTemplate}
                    disabled={isDownloadingTemplate}
                    className="inline-flex items-center gap-1 text-[11px] font-mono text-amber-400 hover:text-amber-300 hover:underline transition-colors"
                  >
                    <Download className="h-3 w-3" />
                    {isDownloadingTemplate ? "Downloading..." : "Download Template (.xlsx)"}
                  </button>
                </div>

                <div className="overflow-x-auto">
                  <table className="w-full text-[10px] font-mono border-collapse border border-white/[0.08] text-center rounded">
                    <thead>
                      <tr className="bg-[#121520] text-amber-300">
                        <th className="py-1 px-2 border border-white/[0.08]">ID</th>
                        <th className="py-1 px-2 border border-white/[0.08]">Pattern</th>
                        <th className="py-1 px-2 border border-white/[0.08]">Subpattern</th>
                        <th className="py-1 px-2 border border-white/[0.08]">Difficulty</th>
                        <th className="py-1 px-2 border border-white/[0.08]">Problem</th>
                        <th className="py-1 px-2 border border-white/[0.08]">LeetCode ID</th>
                      </tr>
                    </thead>
                    <tbody className="text-neutral-400 text-[9.5px]">
                      <tr>
                        <td className="py-1 px-2 border border-white/[0.08]">1</td>
                        <td className="py-1 px-2 border border-white/[0.08]">Arrays</td>
                        <td className="py-1 px-2 border border-white/[0.08]">Prefix Sum</td>
                        <td className="py-1 px-2 border border-white/[0.08]">Easy</td>
                        <td className="py-1 px-2 border border-white/[0.08]">Two Sum</td>
                        <td className="py-1 px-2 border border-white/[0.08]">1</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
                <p className="text-[10px] text-neutral-400 leading-normal">
                  Your Excel sheet must include these exact column headers. Question solve metadata travels across all roadmaps automatically.
                </p>
              </div>

              {/* File Upload Drop Area */}
              <div className="space-y-1.5">
                <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
                  Upload Excel File (.xlsx, .xls) <span className="text-amber-400">*</span>
                </label>
                <div
                  className={`p-5 rounded-xl border border-dashed text-center transition-colors cursor-pointer flex flex-col items-center justify-center gap-2 ${importFile
                      ? "border-amber-400/60 bg-amber-500/[0.06]"
                      : "border-white/[0.15] bg-[#121520]/60 hover:border-amber-400/40 hover:bg-[#151926]/80"
                    }`}
                  onClick={() => document.getElementById("roadmap-file-input")?.click()}
                >
                  <input
                    id="roadmap-file-input"
                    type="file"
                    accept=".xlsx,.xls"
                    className="hidden"
                    onChange={(e) => {
                      if (e.target.files && e.target.files[0]) {
                        setImportFile(e.target.files[0]);
                        setImportError(null);
                      }
                    }}
                  />
                  <div className="p-2.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20">
                    <Upload className="h-5 w-5 stroke-[2]" />
                  </div>
                  {importFile ? (
                    <div>
                      <p className="text-xs font-bold text-white">{importFile.name}</p>
                      <p className="text-[11px] font-mono text-amber-400">
                        {(importFile.size / 1024).toFixed(1)} KB &bull; Ready to import
                      </p>
                    </div>
                  ) : (
                    <div>
                      <p className="text-xs font-semibold text-neutral-200">
                        Click or drag & drop your Excel file here
                      </p>
                      <p className="text-[10px] text-neutral-500 font-mono mt-0.5">
                        Supports .xlsx and .xls workbooks
                      </p>
                    </div>
                  )}
                </div>
              </div>

              {/* Submit Buttons */}
              <div className="pt-3 border-t border-white/[0.08] flex items-center justify-end gap-3 shrink-0">
                <button
                  type="button"
                  onClick={() => setIsImportModalOpen(false)}
                  disabled={isImporting}
                  className="px-4 py-2 rounded-xl text-xs font-semibold text-neutral-400 hover:text-white border border-white/[0.08] transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isImporting || !importFile || !importTitle.trim()}
                  className="px-5 py-2 rounded-xl text-xs font-bold bg-gradient-to-r from-amber-500 to-amber-600 text-black shadow-[0_0_20px_rgba(245,158,11,0.28)] hover:shadow-[0_0_30px_rgba(245,158,11,0.45)] disabled:opacity-50 flex items-center gap-2 transition-all cursor-pointer"
                >
                  {isImporting ? (
                    <>
                      <FileSpreadsheet className="h-4 w-4 animate-spin text-black" />
                      Parsing & Importing...
                    </>
                  ) : (
                    <>
                      <FileSpreadsheet className="h-4 w-4 text-black stroke-[2.2]" />
                      Import & Open Roadmap
                    </>
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* AI Roadmap Creation Modal */}
      {isModalOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md"
          onClick={() => !isGenerating && setIsModalOpen(false)}
        >
          <div
            className="w-full max-w-lg bg-[#0a0d14]/95 backdrop-blur-3xl border border-white/[0.12] rounded-2xl shadow-[inset_0_1px_1px_rgba(255,255,255,0.18),0_20px_60px_rgba(0,0,0,0.8),0_0_35px_rgba(245,158,11,0.12)] overflow-hidden"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="p-5 border-b border-white/[0.08] flex items-center justify-between bg-[#0e111a]/90">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-xl bg-amber-500/15 border border-amber-500/25 text-amber-400">
                  <Sparkles className="h-4 w-4 stroke-[2.4]" />
                </div>
                <div>
                  <h2 className="text-sm font-bold text-white">Generate Custom Roadmap with AI</h2>
                  <p className="text-[11px] text-neutral-400 font-mono">Powered by Google Gemini</p>
                </div>
              </div>
              <button
                onClick={() => !isGenerating && setIsModalOpen(false)}
                disabled={isGenerating}
                className="p-1.5 rounded-lg text-neutral-400 hover:text-white hover:bg-white/[0.05]"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            <form onSubmit={handleGenerateRoadmap} className="p-6 space-y-4 text-xs text-white">
              {genError && (
                <div className="p-3 rounded-xl border border-amber-500/30 bg-amber-500/10 text-white text-[11px]">
                  {genError}
                </div>
              )}

              {/* Target Companies */}
              <div className="space-y-2">
                <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
                  Target Companies (Pick 1 or more)
                </label>
                <div className="flex flex-wrap gap-2">
                  {POPULAR_COMPANIES.map((c) => {
                    const isSelected = selectedCompanies.includes(c);
                    return (
                      <button
                        key={c}
                        type="button"
                        onClick={() => toggleCompany(c)}
                        className={`px-3 py-1.5 rounded-xl border text-xs font-mono transition-all ${isSelected
                            ? "bg-gradient-to-r from-amber-500 to-amber-600 text-black border-amber-500 font-bold shadow-[0_0_12px_rgba(245,158,11,0.25)]"
                            : "bg-[#121520] border-white/[0.08] text-neutral-400 hover:border-amber-400/40 hover:text-white"
                          }`}
                      >
                        {c}
                      </button>
                    );
                  })}
                </div>
              </div>

              {/* Timeline & Daily Hours */}
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5">
                  <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
                    Timeline (Days)
                  </label>
                  <select
                    value={timelineDays}
                    onChange={(e) => setTimelineDays(parseInt(e.target.value))}
                    className="w-full p-2.5 rounded-xl bg-[#121520] border border-white/[0.1] focus:border-amber-400/50 text-white text-xs focus:outline-none transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
                  >
                    <option value={30}>30 Days (Sprint)</option>
                    <option value={45}>45 Days (Focused)</option>
                    <option value={60}>60 Days (Standard FAANG)</option>
                    <option value={90}>90 Days (Deep Mastery)</option>
                  </select>
                </div>

                <div className="space-y-1.5">
                  <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
                    Daily Study (Hours)
                  </label>
                  <select
                    value={dailyHours}
                    onChange={(e) => setDailyHours(parseFloat(e.target.value))}
                    className="w-full p-2.5 rounded-xl bg-[#121520] border border-white/[0.1] focus:border-amber-400/50 text-white text-xs focus:outline-none transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
                  >
                    <option value={1}>1 Hour / Day</option>
                    <option value={2}>2 Hours / Day</option>
                    <option value={3}>3 Hours / Day</option>
                    <option value={4}>4+ Hours / Day</option>
                  </select>
                </div>
              </div>

              {/* Experience Level */}
              <div className="space-y-1.5">
                <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
                  Experience Level
                </label>
                <div className="grid grid-cols-3 gap-2">
                  {(
                    [
                      { key: "beginner", label: "Beginner" },
                      { key: "intermediate", label: "Intermediate" },
                      { key: "advanced", label: "Advanced" },
                    ] as const
                  ).map((lvl) => (
                    <button
                      key={lvl.key}
                      type="button"
                      onClick={() => setExperienceLevel(lvl.key)}
                      className={`py-2 text-center rounded-xl border font-mono text-xs transition-all ${experienceLevel === lvl.key
                          ? "bg-gradient-to-r from-amber-500 to-amber-600 text-black border-amber-500 font-bold shadow-[0_0_12px_rgba(245,158,11,0.25)]"
                          : "bg-[#121520] border-white/[0.08] text-neutral-400 hover:border-amber-400/40 hover:text-white"
                        }`}
                    >
                      {lvl.label}
                    </button>
                  ))}
                </div>
              </div>

              {/* Focus Notes */}
              <div className="space-y-1.5">
                <label className="font-mono text-[11px] text-neutral-300 uppercase tracking-wider block font-semibold">
                  Focus Notes / Weaknesses (Optional)
                </label>
                <input
                  type="text"
                  value={focusNotes}
                  onChange={(e) => setFocusNotes(e.target.value)}
                  placeholder="e.g. Focus on Dynamic Programming and Graphs..."
                  className="w-full p-2.5 rounded-xl bg-[#121520] border border-white/[0.1] focus:border-amber-400/50 text-white placeholder-neutral-500 text-xs focus:outline-none transition-colors shadow-[inset_0_1px_2px_rgba(0,0,0,0.3)]"
                />
              </div>

              {/* Submit Buttons */}
              <div className="pt-3 border-t border-white/[0.08] flex items-center justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  disabled={isGenerating}
                  className="px-4 py-2 rounded-xl text-xs font-semibold text-neutral-400 hover:text-white border border-white/[0.08] transition-colors"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isGenerating}
                  className="px-5 py-2 rounded-xl text-xs font-bold bg-gradient-to-r from-amber-500 to-amber-600 text-black shadow-[0_0_20px_rgba(245,158,11,0.28)] hover:shadow-[0_0_30px_rgba(245,158,11,0.45)] disabled:opacity-50 flex items-center gap-2 transition-all cursor-pointer"
                >
                  {isGenerating ? (
                    <>
                      <Sparkles className="h-4 w-4 text-black animate-spin" />
                      Gemini Generating...
                    </>
                  ) : (
                    "Generate & Activate Roadmap"
                  )}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {roadmapToDelete && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-md"
          onClick={() => !isDeletingRoadmap && setRoadmapToDelete(null)}
        >
          <div
            className="w-full max-w-md bg-[#0a0d14]/95 backdrop-blur-3xl border border-white/[0.12] rounded-2xl shadow-[inset_0_1px_1px_rgba(255,255,255,0.18),0_20px_60px_rgba(0,0,0,0.8),0_0_35px_rgba(244,63,94,0.12)] p-6 space-y-4"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center gap-3">
              <div className="p-2.5 rounded-xl bg-rose-500/15 border border-rose-500/30 text-rose-400">
                <AlertTriangle className="h-5 w-5 stroke-[2.2]" />
              </div>
              <div>
                <h3 className="text-sm font-bold text-white">Delete Custom Roadmap</h3>
                <p className="text-[11px] text-neutral-400 font-mono">This action cannot be undone</p>
              </div>
            </div>

            {deleteError && (
              <div className="p-3 rounded-xl border border-rose-500/30 bg-rose-500/10 text-rose-200 text-xs">
                {deleteError}
              </div>
            )}

            <p className="text-xs text-neutral-300 leading-relaxed">
              Are you sure you want to delete <strong className="text-white font-semibold">&ldquo;{roadmapToDelete.title}&rdquo;</strong>?
              Your solved question records and progress will remain safely intact in the Master Question Bank.
            </p>

            <div className="pt-2 flex items-center justify-end gap-3 border-t border-white/[0.08]">
              <button
                type="button"
                onClick={() => setRoadmapToDelete(null)}
                disabled={isDeletingRoadmap}
                className="px-4 py-2 rounded-xl text-xs font-semibold text-neutral-400 hover:text-white border border-white/[0.08] transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleDeleteRoadmap}
                disabled={isDeletingRoadmap}
                className="px-4 py-2 rounded-xl text-xs font-bold bg-rose-500 hover:bg-rose-600 text-white shadow-[0_0_15px_rgba(244,63,94,0.3)] transition-all cursor-pointer flex items-center gap-1.5"
              >
                {isDeletingRoadmap ? "Deleting..." : "Delete Roadmap"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
