"use client";

import React, { useEffect, useState, useCallback, useRef } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";
import { api } from "@/lib/api";
import {
  Question,
  Pattern,
  Difficulty,
  ImportPreviewResult,
  ImportResult,
  ImportHistory,
  CreateQuestionPayload,
  UpdateQuestionPayload,
} from "@/types";
import {
  ShieldAlert,
  Coins,
  Send,
  CheckCircle2,
  AlertCircle,
  Database,
  Layers,
  FileSpreadsheet,
  UploadCloud,
  Search,
  Plus,
  Edit2,
  Archive,
  RefreshCw,
  Eye,
  Check,
  Clock,
  ExternalLink,
  X,
  Activity,
  ChevronLeft,
  ChevronRight,
  RotateCcw,
} from "lucide-react";

type AdminTab = "catalog" | "import" | "grants" | "system";

export default function AdminPage() {
  const router = useRouter();
  const { user, loading: authLoading } = useAuth();
  const [activeTab, setActiveTab] = useState<AdminTab>("catalog");

  // ==========================================
  // Auth Verification
  // ==========================================
  useEffect(() => {
    if (!authLoading && (!user || user.role !== "ADMIN")) {
      router.push("/dashboard");
    }
  }, [user, authLoading, router]);

  // ==========================================
  // TAB 1: QUESTION CATALOG MANAGEMENT
  // ==========================================
  const [questions, setQuestions] = useState<Question[]>([]);
  const [totalQuestions, setTotalQuestions] = useState(0);
  const [patterns, setPatterns] = useState<Pattern[]>([]);
  const [loadingQuestions, setLoadingQuestions] = useState(false);
  const [questionError, setQuestionError] = useState<string | null>(null);

  // Filter & Pagination States
  const [search, setSearch] = useState("");
  const [difficultyFilter, setDifficultyFilter] = useState<string>("");
  const [patternFilter, setPatternFilter] = useState<string>("");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [page, setPage] = useState(1);
  const pageSize = 25;

  // Add / Edit Modal States
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingQuestion, setEditingQuestion] = useState<Question | null>(null);
  const [formTitle, setFormTitle] = useState("");
  const [formDifficulty, setFormDifficulty] = useState<Difficulty>("MEDIUM");
  const [formLeetCodeId, setFormLeetCodeId] = useState("");
  const [formIsCore, setFormIsCore] = useState(false);
  const [formTimeLimit, setFormTimeLimit] = useState(30);
  const [formEditorial, setFormEditorial] = useState("");
  const [formPatternId, setFormPatternId] = useState("");
  const [isSavingQuestion, setIsSavingQuestion] = useState(false);
  const [modalError, setModalError] = useState<string | null>(null);
  const [archivingId, setArchivingId] = useState<string | null>(null);
  const [toastMessage, setToastMessage] = useState<{ type: "success" | "error"; text: string } | null>(null);

  const fetchPatterns = useCallback(async () => {
    try {
      const res = await api.listPatterns();
      setPatterns(res.patterns || []);
    } catch (err: unknown) {
      console.error("Failed to load patterns:", err);
    }
  }, []);

  const fetchQuestions = useCallback(async () => {
    setLoadingQuestions(true);
    setQuestionError(null);
    try {
      const res = await api.adminListQuestions({
        search: search.trim() || undefined,
        difficulty: difficultyFilter || undefined,
        pattern_id: patternFilter || undefined,
        is_active: statusFilter === "all" ? undefined : statusFilter,
        limit: pageSize,
        offset: (page - 1) * pageSize,
      });
      setQuestions(res.questions || []);
      setTotalQuestions(res.total || 0);
    } catch (err: unknown) {
      const e = err as Error;
      setQuestionError(e.message || "Failed to load questions from database.");
    } finally {
      setLoadingQuestions(false);
    }
  }, [search, difficultyFilter, patternFilter, statusFilter, page]);

  useEffect(() => {
    if (user?.role === "ADMIN") {
      fetchPatterns();
    }
  }, [user, fetchPatterns]);

  useEffect(() => {
    if (user?.role === "ADMIN" && activeTab === "catalog") {
      fetchQuestions();
    }
  }, [user, activeTab, fetchQuestions]);

  const showToast = (type: "success" | "error", text: string) => {
    setToastMessage({ type, text });
    setTimeout(() => setToastMessage(null), 5000);
  };

  const openCreateModal = () => {
    setEditingQuestion(null);
    setFormTitle("");
    setFormDifficulty("MEDIUM");
    setFormLeetCodeId("");
    setFormIsCore(false);
    setFormTimeLimit(30);
    setFormEditorial("");
    setFormPatternId(patterns[0]?.id || "");
    setModalError(null);
    setIsModalOpen(true);
  };

  const openEditModal = (q: Question) => {
    setEditingQuestion(q);
    setFormTitle(q.title);
    setFormDifficulty(q.difficulty);
    setFormLeetCodeId(q.leetcode_id || "");
    setFormIsCore(q.is_core);
    setFormTimeLimit(q.sample_time_limit_mins || 30);
    setFormEditorial(q.editorial_summary || "");
    setFormPatternId(q.patterns && q.patterns.length > 0 ? q.patterns[0].id : "");
    setModalError(null);
    setIsModalOpen(true);
  };

  const handleSaveQuestion = async (e: React.FormEvent) => {
    e.preventDefault();
    setModalError(null);
    setIsSavingQuestion(true);

    try {
      if (editingQuestion) {
        const payload: UpdateQuestionPayload = {
          title: formTitle.trim(),
          difficulty: formDifficulty,
          leetcode_id: formLeetCodeId.trim() || undefined,
          is_core: formIsCore,
          sample_time_limit_mins: formTimeLimit,
          editorial_summary: formEditorial.trim() || undefined,
          pattern_ids: formPatternId ? [formPatternId] : [],
        };
        await api.adminUpdateQuestion(editingQuestion.id, payload);
        showToast("success", `Updated question "${formTitle.trim()}".`);
      } else {
        const payload: CreateQuestionPayload = {
          title: formTitle.trim(),
          difficulty: formDifficulty,
          leetcode_id: formLeetCodeId.trim() || undefined,
          is_core: formIsCore,
          sample_time_limit_mins: formTimeLimit,
          editorial_summary: formEditorial.trim() || undefined,
          pattern_ids: formPatternId ? [formPatternId] : [],
        };
        await api.adminCreateQuestion(payload);
        showToast("success", `Created new question "${formTitle.trim()}".`);
      }
      setIsModalOpen(false);
      fetchQuestions();
      fetchPatterns();
    } catch (err: unknown) {
      const e = err as Error;
      setModalError(e.message || "Failed to save question.");
    } finally {
      setIsSavingQuestion(false);
    }
  };

  const handleToggleArchive = async (q: Question) => {
    const nextState = q.is_active === false; // If currently archived/false, next state is active/true
    setArchivingId(q.id);
    try {
      await api.adminToggleArchive(q.id, nextState);
      showToast(
        "success",
        nextState
          ? `Question "${q.title}" reactivated and restored to candidate catalog.`
          : `Question "${q.title}" archived. Removed from active candidate catalog (historical attempts preserved).`
      );
      fetchQuestions();
    } catch (err: unknown) {
      const e = err as Error;
      showToast("error", e.message || "Failed to update archival status.");
    } finally {
      setArchivingId(null);
    }
  };

  // ==========================================
  // TAB 2: BULK EXCEL WORKBOOK IMPORTER
  // ==========================================
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [customPath, setCustomPath] = useState("F:\\DSA\\FAANG_DSA_Master_Plan_v2.xlsx");
  const [useFilePathInput, setUseFilePathInput] = useState(false);
  const [isPreviewing, setIsPreviewing] = useState(false);
  const [previewResult, setPreviewResult] = useState<ImportPreviewResult | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);
  const [confirmGate, setConfirmGate] = useState(false);
  const [isCommitting, setIsCommitting] = useState(false);
  const [commitResult, setCommitResult] = useState<ImportResult | null>(null);
  const [commitError, setCommitError] = useState<string | null>(null);
  const [importHistory, setImportHistory] = useState<ImportHistory[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const fetchHistory = useCallback(async () => {
    setLoadingHistory(true);
    try {
      const res = await api.adminListImportHistory(10, 0);
      setImportHistory(res.history || []);
    } catch (err: unknown) {
      console.error("Failed to fetch import history:", err);
    } finally {
      setLoadingHistory(false);
    }
  }, []);

  useEffect(() => {
    if (user?.role === "ADMIN" && activeTab === "import") {
      fetchHistory();
    }
  }, [user, activeTab, fetchHistory]);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (e.target.files && e.target.files.length > 0) {
      setSelectedFile(e.target.files[0]);
      setPreviewResult(null);
      setCommitResult(null);
      setPreviewError(null);
      setCommitError(null);
      setConfirmGate(false);
    }
  };

  const handleRunPreview = async () => {
    setPreviewError(null);
    setCommitError(null);
    setCommitResult(null);
    setConfirmGate(false);
    setIsPreviewing(true);

    try {
      const payload = useFilePathInput ? customPath.trim() : selectedFile;
      if (!payload) {
        throw new Error("Please select an Excel file (.xlsx) or provide a valid host file path.");
      }
      const res = await api.adminImportPreview(payload);
      setPreviewResult(res);
      showToast("success", `Validated ${res.total_rows_read} rows from sheet "${res.detected_sheet}".`);
    } catch (err: unknown) {
      const e = err as Error;
      setPreviewError(e.message || "Failed to preview and validate Excel workbook.");
    } finally {
      setIsPreviewing(false);
    }
  };

  const handleCommitImport = async () => {
    if (!confirmGate) {
      showToast("error", "Please confirm that you have reviewed the planned operations before importing.");
      return;
    }

    setCommitError(null);
    setIsCommitting(true);

    try {
      const payload = useFilePathInput ? customPath.trim() : selectedFile;
      if (!payload) {
        throw new Error("No import file or path specified.");
      }
      const res = await api.adminImportCommit(payload);
      setCommitResult(res);
      showToast(
        "success",
        `Import committed successfully! Created ${res.created_questions} questions, linked ${res.cross_listed_rows} cross-listed patterns.`
      );
      fetchHistory();
      fetchPatterns();
    } catch (err: unknown) {
      const e = err as Error;
      setCommitError(e.message || "Failed to commit import into database.");
    } finally {
      setIsCommitting(false);
    }
  };

  // ==========================================
  // TAB 3: PROMOTIONAL CREDITS GRANT
  // ==========================================
  const [targetUserId, setTargetUserId] = useState("");
  const [grantAmount, setGrantAmount] = useState(50);
  const [grantReason, setGrantReason] = useState("");
  const [isSubmittingGrant, setIsSubmittingGrant] = useState(false);
  const [grantSuccessMsg, setGrantSuccessMsg] = useState<string | null>(null);
  const [grantErrorMsg, setGrantErrorMsg] = useState<string | null>(null);

  const handleGrant = async (e: React.FormEvent) => {
    e.preventDefault();
    setGrantSuccessMsg(null);
    setGrantErrorMsg(null);
    setIsSubmittingGrant(true);

    try {
      const res = await api.adminGrantCredits(targetUserId.trim(), Number(grantAmount), grantReason.trim());
      setGrantSuccessMsg(
        `Successfully granted ${grantAmount} credits to user ${targetUserId}. Entry ID: ${res.entry.id}`
      );
      setGrantReason("");
    } catch (err: unknown) {
      const e = err as Error;
      setGrantErrorMsg(e.message || "Failed to grant promotional credits.");
    } finally {
      setIsSubmittingGrant(false);
    }
  };

  // ==========================================
  // Helper Badges
  // ==========================================
  const getDifficultyBadge = (diff: string) => {
    switch (diff) {
      case "EASY":
        return "bg-zinc-900 text-zinc-300 border border-zinc-800";
      case "MEDIUM":
        return "bg-[#F59E0B]/15 text-[#F59E0B] border border-[#F59E0B]/30";
      case "HARD":
        return "bg-[#F59E0B] text-black border border-[#F59E0B] font-bold";
      default:
        return "bg-zinc-900 text-zinc-400 border border-zinc-800";
    }
  };

  const getActionBadge = (action: string) => {
    switch (action) {
      case "CREATE":
        return "bg-[#F59E0B]/15 text-[#F59E0B] border border-[#F59E0B]/30";
      case "CROSS_LIST":
        return "bg-zinc-900 text-zinc-200 border border-zinc-800";
      case "UPDATE":
        return "bg-[#F59E0B]/25 text-[#F59E0B] border border-[#F59E0B]/40";
      case "UNCHANGED":
        return "bg-zinc-900 text-zinc-500 border border-zinc-800";
      default:
        return "bg-zinc-900 text-zinc-400 border border-zinc-800";
    }
  };

  if (authLoading || !user || user.role !== "ADMIN") {
    return (
      <div className="max-w-4xl mx-auto px-4 py-20 text-center text-zinc-500 font-mono text-sm flex items-center justify-center gap-2">
        <Activity className="h-4 w-4 animate-spin text-[#F59E0B]" />
        Verifying administrative privileges...
      </div>
    );
  }

  const totalPages = Math.ceil(totalQuestions / pageSize) || 1;

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 w-full space-y-8">
      {/* Toast Notification */}
      {toastMessage && (
        <div
          className={`fixed bottom-6 right-6 z-50 px-4 py-3 rounded-xl border shadow-xl flex items-center gap-3 text-xs font-medium transition-all animate-in fade-in slide-in-from-bottom-5 ${
            toastMessage.type === "success"
              ? "bg-zinc-950 border-[#F59E0B]/40 text-white shadow-[#F59E0B]/10"
              : "bg-zinc-950 border-zinc-800 text-zinc-300 shadow-black"
          }`}
        >
          {toastMessage.type === "success" ? (
            <CheckCircle2 className="h-4 w-4 text-[#F59E0B] shrink-0" />
          ) : (
            <AlertCircle className="h-4 w-4 text-[#F59E0B] shrink-0" />
          )}
          <span>{toastMessage.text}</span>
          <button
            onClick={() => setToastMessage(null)}
            className="ml-2 text-zinc-400 hover:text-white"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-zinc-900 pb-6">
        <div>
          <div className="flex items-center gap-2 text-xs font-mono text-[#F59E0B] mb-1">
            <ShieldAlert className="h-4 w-4" />
            <span>ADMINISTRATIVE COMMAND CENTER</span>
          </div>
          <h1 className="text-2xl sm:text-3xl font-extrabold text-white tracking-tight">
            Question Bank &amp; Catalog Management
          </h1>
          <p className="text-sm text-zinc-400 mt-0.5">
            Bulk import from master Excel workbooks, validate question integrity, and administer the active candidate catalog.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <div className="px-3 py-1.5 rounded-xl bg-[#F59E0B]/10 border border-[#F59E0B]/20 text-xs font-mono text-[#F59E0B]">
            Role: <strong className="text-white">SUPER_ADMIN</strong>
          </div>
        </div>
      </div>

      {/* Tabs Navigation */}
      <div className="flex items-center gap-2 border-b border-zinc-900 pb-1 overflow-x-auto scrollbar-none">
        <button
          onClick={() => setActiveTab("catalog")}
          className={`flex items-center gap-2 px-4 py-2.5 rounded-xl text-xs font-bold font-mono transition-all cursor-pointer ${
            activeTab === "catalog"
              ? "bg-[#F59E0B]/15 text-[#F59E0B] border border-[#F59E0B]/30 shadow-sm"
              : "text-zinc-400 hover:text-white hover:bg-zinc-900 border border-transparent"
          }`}
        >
          <Layers className="h-4 w-4" />
          Question Bank Catalog
          <span className="ml-1 px-1.5 py-0.2 rounded-md bg-zinc-900 text-[10px] text-zinc-300">
            {totalQuestions}
          </span>
        </button>

        <button
          onClick={() => setActiveTab("import")}
          className={`flex items-center gap-2 px-4 py-2.5 rounded-xl text-xs font-bold font-mono transition-all cursor-pointer ${
            activeTab === "import"
              ? "bg-[#F59E0B]/15 text-[#F59E0B] border border-[#F59E0B]/30 shadow-sm"
              : "text-zinc-400 hover:text-white hover:bg-zinc-900 border border-transparent"
          }`}
        >
          <FileSpreadsheet className="h-4 w-4" />
          Bulk Excel Import
        </button>

        <button
          onClick={() => setActiveTab("grants")}
          className={`flex items-center gap-2 px-4 py-2.5 rounded-xl text-xs font-bold font-mono transition-all cursor-pointer ${
            activeTab === "grants"
              ? "bg-[#F59E0B]/15 text-[#F59E0B] border border-[#F59E0B]/30 shadow-sm"
              : "text-zinc-400 hover:text-white hover:bg-zinc-900 border border-transparent"
          }`}
        >
          <Coins className="h-4 w-4" />
          Promotional Grants
        </button>

        <button
          onClick={() => setActiveTab("system")}
          className={`flex items-center gap-2 px-4 py-2.5 rounded-xl text-xs font-bold font-mono transition-all cursor-pointer ${
            activeTab === "system"
              ? "bg-[#F59E0B]/15 text-[#F59E0B] border border-[#F59E0B]/30 shadow-sm"
              : "text-zinc-400 hover:text-white hover:bg-zinc-900 border border-transparent"
          }`}
        >
          <Database className="h-4 w-4" />
          System Diagnostics
        </button>
      </div>

      {/* ========================================================================= */}
      {/* TAB 1: QUESTION CATALOG MANAGEMENT VIEW                                  */}
      {/* ========================================================================= */}
      {activeTab === "catalog" && (
        <div className="space-y-6">
          {/* Controls Bar */}
          <div className="p-4 rounded-2xl bg-zinc-950 border border-zinc-900 flex flex-col lg:flex-row gap-4 items-stretch lg:items-center justify-between">
            {/* Search Input */}
            <div className="relative flex-1">
              <Search className="absolute left-3.5 top-3 h-4 w-4 text-zinc-500" />
              <input
                type="text"
                value={search}
                onChange={(e) => {
                  setSearch(e.target.value);
                  setPage(1);
                }}
                placeholder="Search by problem title or LeetCode ID..."
                className="w-full pl-10 pr-4 py-2 rounded-xl bg-black border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:outline-none focus:border-[#F59E0B] text-xs transition-colors"
              />
            </div>

            {/* Filters */}
            <div className="flex flex-wrap items-center gap-3">
              {/* Difficulty Filter */}
              <div className="flex items-center gap-1.5">
                <span className="text-[11px] font-mono text-zinc-400 hidden sm:inline">Difficulty:</span>
                <select
                  value={difficultyFilter}
                  onChange={(e) => {
                    setDifficultyFilter(e.target.value);
                    setPage(1);
                  }}
                  className="px-3 py-2 rounded-xl bg-black border border-zinc-800 text-zinc-200 text-xs focus:outline-none focus:border-[#F59E0B]"
                >
                  <option value="">All Difficulties</option>
                  <option value="EASY">Easy</option>
                  <option value="MEDIUM">Medium</option>
                  <option value="HARD">Hard</option>
                </select>
              </div>

              {/* Pattern Filter */}
              <div className="flex items-center gap-1.5">
                <span className="text-[11px] font-mono text-zinc-400 hidden sm:inline">Pattern:</span>
                <select
                  value={patternFilter}
                  onChange={(e) => {
                    setPatternFilter(e.target.value);
                    setPage(1);
                  }}
                  className="px-3 py-2 rounded-xl bg-black border border-zinc-800 text-zinc-200 text-xs focus:outline-none focus:border-[#F59E0B] max-w-[180px]"
                >
                  <option value="">All Patterns ({patterns.length})</option>
                  {patterns.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* Status Filter */}
              <div className="flex items-center gap-1.5">
                <span className="text-[11px] font-mono text-zinc-400 hidden sm:inline">Catalog Status:</span>
                <select
                  value={statusFilter}
                  onChange={(e) => {
                    setStatusFilter(e.target.value);
                    setPage(1);
                  }}
                  className="px-3 py-2 rounded-xl bg-black border border-zinc-800 text-zinc-200 text-xs focus:outline-none focus:border-[#F59E0B]"
                >
                  <option value="all">All Statuses</option>
                  <option value="true">Active Only (Candidate Visible)</option>
                  <option value="false">Archived Only (Hidden)</option>
                </select>
              </div>

              {/* Add Question Button */}
              <button
                onClick={openCreateModal}
                className="flex items-center gap-2 px-4 py-2 rounded-xl bg-[#F59E0B] hover:bg-[#F59E0B]/90 text-black font-bold text-xs transition-all shadow-md shadow-[#F59E0B]/20 cursor-pointer"
              >
                <Plus className="h-3.5 w-3.5" />
                Add Question
              </button>
            </div>
          </div>

          {/* Error Banner */}
          {questionError && (
            <div className="p-4 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-300 text-xs flex items-center justify-between">
              <div className="flex items-center gap-2">
                <AlertCircle className="h-4 w-4 text-[#F59E0B] shrink-0" />
                <span>{questionError}</span>
              </div>
              <button
                onClick={fetchQuestions}
                className="px-2.5 py-1 rounded-lg bg-zinc-900 hover:bg-zinc-850 text-[#F59E0B] font-mono text-[10px] cursor-pointer"
              >
                Retry
              </button>
            </div>
          )}

          {/* Questions Table */}
          <div className="rounded-2xl bg-zinc-950 border border-zinc-900 overflow-hidden shadow-xl">
            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="border-b border-zinc-900 bg-black text-[11px] font-mono uppercase text-zinc-500 tracking-wider">
                    <th className="py-3.5 px-4 w-16">LC ID</th>
                    <th className="py-3.5 px-4">Problem Title</th>
                    <th className="py-3.5 px-4 w-28">Difficulty</th>
                    <th className="py-3.5 px-4">Primary Pattern</th>
                    <th className="py-3.5 px-4 w-28">Status</th>
                    <th className="py-3.5 px-4 w-36 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-900 text-xs">
                  {loadingQuestions ? (
                    <tr>
                      <td colSpan={6} className="py-12 text-center text-zinc-500 font-mono">
                        <div className="flex items-center justify-center gap-2">
                          <Activity className="h-4 w-4 animate-spin text-[#F59E0B]" />
                          Loading question catalog...
                        </div>
                      </td>
                    </tr>
                  ) : questions.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="py-12 text-center text-zinc-500 font-mono">
                        No questions matched the active filters or search query.
                      </td>
                    </tr>
                  ) : (
                    questions.map((q) => {
                      const primaryPattern =
                        q.patterns && q.patterns.length > 0 ? q.patterns[0].name : "Uncategorized";
                      const additionalPatternsCount =
                        q.patterns && q.patterns.length > 1 ? q.patterns.length - 1 : 0;
                      const isActive = q.is_active !== false;

                      return (
                        <tr
                          key={q.id}
                          className="hover:bg-zinc-900/40 transition-colors group"
                        >
                          {/* LeetCode ID */}
                          <td className="py-3.5 px-4 font-mono text-zinc-400 text-[11px]">
                            {q.leetcode_id ? `#${q.leetcode_id}` : "—"}
                          </td>

                          {/* Problem Title & Slug */}
                          <td className="py-3.5 px-4">
                            <div className="flex items-center gap-2">
                              <span className="font-semibold text-zinc-100 group-hover:text-[#F59E0B] transition-colors">
                                {q.title}
                              </span>
                              {q.is_core && (
                                <span className="px-1.5 py-0.5 rounded bg-[#F59E0B]/15 border border-[#F59E0B]/30 text-[#F59E0B] text-[10px] font-mono">
                                  CORE
                                </span>
                              )}
                              {q.external_url && (
                                <a
                                  href={q.external_url}
                                  target="_blank"
                                  rel="noopener noreferrer"
                                  className="text-zinc-600 hover:text-zinc-300 transition-colors"
                                  title="Open in LeetCode"
                                >
                                  <ExternalLink className="h-3 w-3" />
                                </a>
                              )}
                            </div>
                            <div className="text-[11px] text-zinc-500 font-mono">{q.slug}</div>
                          </td>

                          {/* Difficulty */}
                          <td className="py-3.5 px-4">
                            <span
                              className={`px-2 py-0.5 rounded-full text-[10px] font-mono font-bold border ${getDifficultyBadge(
                                q.difficulty
                              )}`}
                            >
                              {q.difficulty}
                            </span>
                          </td>

                          {/* Primary Pattern & Tags */}
                          <td className="py-3.5 px-4">
                            <div className="flex items-center gap-1.5 flex-wrap">
                              <span className="px-2 py-0.5 rounded-md bg-zinc-900 border border-zinc-800 text-zinc-300 text-[11px] font-medium">
                                {primaryPattern}
                              </span>
                              {additionalPatternsCount > 0 && (
                                <span
                                  className="px-1.5 py-0.5 rounded-md bg-zinc-900 border border-zinc-800 text-zinc-400 text-[10px] font-mono"
                                  title={q.patterns?.map((p) => p.name).join(", ")}
                                >
                                  +{additionalPatternsCount} more
                                </span>
                              )}
                            </div>
                          </td>

                          {/* Active / Archived Status */}
                          <td className="py-3.5 px-4">
                            {isActive ? (
                              <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-[#F59E0B]/15 border border-[#F59E0B]/30 text-[#F59E0B] text-[10px] font-mono font-medium">
                                <span className="h-1.5 w-1.5 rounded-full bg-[#F59E0B] animate-pulse" />
                                ACTIVE
                              </span>
                            ) : (
                              <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full bg-zinc-900 border border-zinc-800 text-zinc-500 text-[10px] font-mono font-medium">
                                ARCHIVED
                              </span>
                            )}
                          </td>

                          {/* Actions */}
                          <td className="py-3.5 px-4 text-right">
                            <div className="flex items-center justify-end gap-1.5">
                              {/* Edit Button */}
                              <button
                                onClick={() => openEditModal(q)}
                                className="p-1.5 rounded-lg bg-zinc-900 hover:bg-[#F59E0B]/20 hover:text-[#F59E0B] text-zinc-400 transition-colors cursor-pointer"
                                title="Edit Question Details"
                              >
                                <Edit2 className="h-3.5 w-3.5" />
                              </button>

                              {/* Archive / Restore Button */}
                              <button
                                onClick={() => handleToggleArchive(q)}
                                disabled={archivingId === q.id}
                                className={`p-1.5 rounded-lg transition-colors cursor-pointer ${
                                  isActive
                                    ? "bg-zinc-900 hover:bg-zinc-850 text-zinc-400 hover:text-white"
                                    : "bg-[#F59E0B]/15 hover:bg-[#F59E0B]/25 text-[#F59E0B]"
                                }`}
                                title={
                                  isActive
                                    ? "Archive question (hide from catalog)"
                                    : "Restore question (reactivate in catalog)"
                                }
                              >
                                {archivingId === q.id ? (
                                  <Activity className="h-3.5 w-3.5 animate-spin" />
                                ) : isActive ? (
                                  <Archive className="h-3.5 w-3.5" />
                                ) : (
                                  <RotateCcw className="h-3.5 w-3.5" />
                                )}
                              </button>
                            </div>
                          </td>
                        </tr>
                      );
                    })
                  )}
                </tbody>
              </table>
            </div>

            {/* Pagination Controls */}
            <div className="p-4 border-t border-zinc-900 bg-black/40 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs">
              <div className="text-zinc-400 font-mono text-[11px]">
                Showing {questions.length > 0 ? (page - 1) * pageSize + 1 : 0} to{" "}
                {Math.min(page * pageSize, totalQuestions)} of {totalQuestions} total problems
              </div>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page <= 1 || loadingQuestions}
                  className="px-3 py-1.5 rounded-xl bg-zinc-900 hover:bg-zinc-850 disabled:opacity-40 disabled:hover:bg-zinc-900 text-zinc-200 font-mono text-xs flex items-center gap-1 transition-colors cursor-pointer"
                >
                  <ChevronLeft className="h-3.5 w-3.5" />
                  Prev
                </button>
                <span className="font-mono text-xs text-zinc-300 px-2">
                  Page {page} of {totalPages}
                </span>
                <button
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                  disabled={page >= totalPages || loadingQuestions}
                  className="px-3 py-1.5 rounded-xl bg-zinc-900 hover:bg-zinc-850 disabled:opacity-40 disabled:hover:bg-zinc-900 text-zinc-200 font-mono text-xs flex items-center gap-1 transition-colors cursor-pointer"
                >
                  Next
                  <ChevronRight className="h-3.5 w-3.5" />
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 2: BULK EXCEL IMPORT VIEW                                             */}
      {/* ========================================================================= */}
      {activeTab === "import" && (
        <div className="space-y-8">
          {/* Top: Upload & Configuration Card */}
          <div className="p-6 rounded-2xl bg-zinc-950 border border-zinc-900 space-y-6">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <div>
                <h2 className="text-base font-bold text-white flex items-center gap-2">
                  <FileSpreadsheet className="h-5 w-5 text-[#F59E0B]" />
                  Master DSA Question Bank Excel Import
                </h2>
                <p className="text-xs text-zinc-400 mt-1">
                  Upload an Excel workbook (<code className="text-[#F59E0B]">.xlsx</code> or{" "}
                  <code className="text-[#F59E0B]">.xls</code>) to preview, validate, and transactional-commit questions into PostgreSQL.
                </p>
              </div>

              {/* Source Switcher */}
              <div className="flex items-center gap-2 bg-black p-1 rounded-xl border border-zinc-800 self-start">
                <button
                  onClick={() => setUseFilePathInput(false)}
                  className={`px-3 py-1 rounded-lg text-xs font-mono transition-all cursor-pointer ${
                    !useFilePathInput
                      ? "bg-[#F59E0B]/15 text-[#F59E0B] border border-[#F59E0B]/30 font-bold"
                      : "text-zinc-400 hover:text-white border border-transparent"
                  }`}
                >
                  Browser Upload
                </button>
                <button
                  onClick={() => setUseFilePathInput(true)}
                  className={`px-3 py-1 rounded-lg text-xs font-mono transition-all cursor-pointer ${
                    useFilePathInput
                      ? "bg-[#F59E0B]/15 text-[#F59E0B] border border-[#F59E0B]/30 font-bold"
                      : "text-zinc-400 hover:text-white border border-transparent"
                  }`}
                >
                  Host File Path
                </button>
              </div>
            </div>

            {/* Input Options */}
            {!useFilePathInput ? (
              /* Drag & Drop Upload Zone */
              <div
                onClick={() => fileInputRef.current?.click()}
                className={`border-2 border-dashed rounded-2xl p-8 text-center cursor-pointer transition-all ${
                  selectedFile
                    ? "border-[#F59E0B]/60 bg-[#F59E0B]/5"
                    : "border-zinc-800 hover:border-[#F59E0B]/30 bg-black/40 hover:bg-black/60"
                }`}
              >
                <input
                  ref={fileInputRef}
                  type="file"
                  accept=".xlsx,.xls"
                  onChange={handleFileChange}
                  className="hidden"
                />
                <div className="flex flex-col items-center justify-center gap-3">
                  <div className="h-12 w-12 rounded-xl bg-zinc-900 border border-zinc-800 flex items-center justify-center text-[#F59E0B]">
                    <UploadCloud className="h-6 w-6" />
                  </div>
                  <div>
                    {selectedFile ? (
                      <div className="space-y-1">
                        <span className="font-bold text-sm text-[#F59E0B] font-mono">
                          {selectedFile.name}
                        </span>
                        <div className="text-xs text-zinc-400 font-mono">
                          {(selectedFile.size / 1024).toFixed(1)} KB — Ready for validation
                        </div>
                      </div>
                    ) : (
                      <div className="space-y-1">
                        <span className="font-semibold text-sm text-zinc-200">
                          Click to select workbook or drag and drop here
                        </span>
                        <div className="text-xs text-zinc-500 font-mono">
                          Supports FAANG_DSA_Master_Plan_v2.xlsx (.xlsx format)
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              </div>
            ) : (
              /* Host File Path Input */
              <div className="space-y-2">
                <label className="block text-xs font-mono text-zinc-300 uppercase">
                  Server/Host Absolute File Path
                </label>
                <input
                  type="text"
                  value={customPath}
                  onChange={(e) => setCustomPath(e.target.value)}
                  placeholder="e.g. F:\DSA\FAANG_DSA_Master_Plan_v2.xlsx"
                  className="w-full px-3.5 py-2.5 rounded-xl bg-black border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:outline-none focus:border-[#F59E0B] text-xs font-mono transition-colors"
                />
                <p className="text-[11px] text-zinc-500 font-mono">
                  The backend reads this path directly from the local filesystem or mounted volume.
                </p>
              </div>
            )}

            {/* Preview Action Button */}
            <div className="flex items-center justify-between pt-2">
              <div className="text-xs text-zinc-400 font-mono">
                {selectedFile || useFilePathInput ? (
                  <span>Target: <strong className="text-white">{useFilePathInput ? customPath : selectedFile?.name}</strong></span>
                ) : (
                  <span>Please select a file to begin preview.</span>
                )}
              </div>

              <button
                onClick={handleRunPreview}
                disabled={isPreviewing || (!selectedFile && !useFilePathInput)}
                className="flex items-center gap-2 py-2.5 px-5 rounded-xl bg-[#F59E0B] hover:bg-[#F59E0B]/90 disabled:opacity-50 text-black font-bold text-xs transition-all shadow-md shadow-[#F59E0B]/20 cursor-pointer"
              >
                {isPreviewing ? (
                  <>
                    <Activity className="h-4 w-4 animate-spin text-black" />
                    Parsing &amp; Validating Workbook...
                  </>
                ) : (
                  <>
                    <Eye className="h-4 w-4" />
                    Preview &amp; Validate Workbook
                  </>
                )}
              </button>
            </div>

            {/* Preview Error Banner */}
            {previewError && (
              <div className="p-4 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-300 text-xs flex items-start gap-2.5">
                <AlertCircle className="h-4 w-4 text-[#F59E0B] shrink-0 mt-0.5" />
                <span>{previewError}</span>
              </div>
            )}
          </div>

          {/* Validation Scorecard & Sample Rows (After Preview) */}
          {previewResult && (
            <div className="p-6 rounded-2xl bg-zinc-950 border border-zinc-900 space-y-6 animate-in fade-in slide-in-from-top-4">
              {/* Header */}
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-zinc-900 pb-4">
                <div>
                  <div className="flex items-center gap-2 text-xs font-mono text-[#F59E0B] mb-1">
                    <CheckCircle2 className="h-4 w-4" />
                    <span>PREVIEW VALIDATION SCORECARD</span>
                  </div>
                  <h3 className="text-lg font-bold text-white">
                    Sheet &ldquo;{previewResult.detected_sheet}&rdquo; Analysis
                  </h3>
                </div>

                <div className="flex items-center gap-2 text-xs font-mono">
                  <span className="text-zinc-400">Total Rows:</span>
                  <span className="px-2.5 py-1 rounded-lg bg-black border border-zinc-800 text-white font-bold">
                    {previewResult.total_rows_read}
                  </span>
                </div>
              </div>

              {/* Metrics Grid */}
              <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
                <div className="p-3.5 rounded-xl bg-black border border-zinc-850 space-y-1">
                  <span className="text-[10px] uppercase font-mono text-zinc-400">Valid Rows</span>
                  <div className="text-xl font-bold font-mono text-[#F59E0B]">
                    {previewResult.valid_rows}
                  </div>
                </div>

                <div className="p-3.5 rounded-xl bg-black border border-zinc-850 space-y-1">
                  <span className="text-[10px] uppercase font-mono text-zinc-400">Invalid Rows</span>
                  <div
                    className={`text-xl font-bold font-mono ${
                      previewResult.invalid_rows > 0 ? "text-white" : "text-zinc-500"
                    }`}
                  >
                    {previewResult.invalid_rows}
                  </div>
                </div>

                <div className="p-3.5 rounded-xl bg-black border border-zinc-850 space-y-1">
                  <span className="text-[10px] uppercase font-mono text-zinc-400">Unique Questions</span>
                  <div className="text-xl font-bold font-mono text-white">
                    {previewResult.unique_questions}
                  </div>
                </div>

                <div className="p-3.5 rounded-xl bg-black border border-zinc-850 space-y-1">
                  <span className="text-[10px] uppercase font-mono text-zinc-400">Cross-Listed Links</span>
                  <div className="text-xl font-bold font-mono text-zinc-200">
                    {previewResult.cross_listed_rows}
                  </div>
                </div>

                <div className="p-3.5 rounded-xl bg-black border border-zinc-850 space-y-1">
                  <span className="text-[10px] uppercase font-mono text-zinc-400">Planned Creates</span>
                  <div className="text-xl font-bold font-mono text-[#F59E0B]">
                    {previewResult.planned_creates}
                  </div>
                </div>

                <div className="p-3.5 rounded-xl bg-black border border-zinc-850 space-y-1">
                  <span className="text-[10px] uppercase font-mono text-zinc-400">Planned Updates</span>
                  <div className="text-xl font-bold font-mono text-zinc-300">
                    {previewResult.planned_updates}
                  </div>
                </div>
              </div>

              {/* Row Validation Errors (if any) */}
              {previewResult.row_errors && previewResult.row_errors.length > 0 && (
                <div className="p-4 rounded-xl bg-zinc-950 border border-zinc-800 space-y-2">
                  <div className="flex items-center gap-2 text-[#F59E0B] text-xs font-bold font-mono">
                    <AlertCircle className="h-4 w-4" />
                    <span>Row Validation Errors ({previewResult.row_errors.length})</span>
                  </div>
                  <div className="max-h-40 overflow-y-auto space-y-1 text-[11px] font-mono text-zinc-300">
                    {previewResult.row_errors.map((err, idx) => (
                      <div key={idx} className="flex items-center gap-2">
                        <span className="px-1.5 py-0.5 rounded bg-zinc-900 text-[#F59E0B] border border-zinc-800">
                          Row {err.row}
                        </span>
                        <span className="text-zinc-500">[{err.column}]:</span>
                        <span>{err.message}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Sample Rows Preview Table */}
              <div className="space-y-3">
                <div className="flex items-center justify-between text-xs">
                  <span className="font-bold text-zinc-200">
                    Sample Rows Preview (First {previewResult.sample_rows.length} rows)
                  </span>
                  <span className="text-zinc-500 font-mono text-[11px]">
                    Action reflects proposed database mutation
                  </span>
                </div>

                <div className="rounded-xl border border-zinc-900 overflow-hidden bg-black">
                  <table className="w-full text-left border-collapse text-xs">
                    <thead>
                      <tr className="border-b border-zinc-900 bg-zinc-950 text-[10px] font-mono uppercase text-zinc-500">
                        <th className="py-2.5 px-3 w-14">Row #</th>
                        <th className="py-2.5 px-3 w-28">Action</th>
                        <th className="py-2.5 px-3">Title</th>
                        <th className="py-2.5 px-3 w-20">LC ID</th>
                        <th className="py-2.5 px-3 w-24">Difficulty</th>
                        <th className="py-2.5 px-3">Primary Pattern</th>
                        <th className="py-2.5 px-3">Subpattern</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-zinc-900 text-[11px]">
                      {previewResult.sample_rows.map((row) => (
                        <tr key={row.row_number} className="hover:bg-zinc-900/40">
                          <td className="py-2 px-3 font-mono text-zinc-500">{row.row_number}</td>
                          <td className="py-2 px-3">
                            <span
                              className={`px-1.5 py-0.5 rounded text-[10px] font-mono font-bold border ${getActionBadge(
                                row.action
                              )}`}
                            >
                              {row.action}
                            </span>
                          </td>
                          <td className="py-2 px-3 font-semibold text-zinc-200">{row.title}</td>
                          <td className="py-2 px-3 font-mono text-zinc-400">
                            {row.leetcode_id ? `#${row.leetcode_id}` : "—"}
                          </td>
                          <td className="py-2 px-3">
                            <span
                              className={`px-1.5 py-0.5 rounded-full text-[9px] font-mono font-bold border ${getDifficultyBadge(
                                row.difficulty
                              )}`}
                            >
                              {row.difficulty}
                            </span>
                          </td>
                          <td className="py-2 px-3 text-zinc-300">{row.pattern}</td>
                          <td className="py-2 px-3 text-zinc-500">{row.subpattern || "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Explicit Confirmation Gate */}
              <div className="pt-4 border-t border-zinc-900 space-y-4">
                <label className="flex items-start gap-3 cursor-pointer p-4 rounded-xl bg-black border border-zinc-900 hover:border-zinc-800 transition-colors">
                  <input
                    type="checkbox"
                    checked={confirmGate}
                    onChange={(e) => setConfirmGate(e.target.checked)}
                    className="mt-0.5 h-4 w-4 rounded bg-zinc-950 border-zinc-700 text-[#F59E0B] focus:ring-[#F59E0B]/20"
                  />
                  <div className="text-xs space-y-1">
                    <span className="font-bold text-white block">
                      I have reviewed the validation scorecard and confirm committing these changes to PostgreSQL.
                    </span>
                    <span className="text-zinc-400 block leading-relaxed">
                      This transaction will upsert questions, establish pattern links, preserve active candidate progress, and record an audit log in the import history.
                    </span>
                  </div>
                </label>

                {commitError && (
                  <div className="p-4 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-300 text-xs flex items-start gap-2.5">
                    <AlertCircle className="h-4 w-4 text-[#F59E0B] shrink-0 mt-0.5" />
                    <span>{commitError}</span>
                  </div>
                )}

                <div className="flex items-center justify-end gap-3">
                  <button
                    onClick={() => {
                      setPreviewResult(null);
                      setConfirmGate(false);
                    }}
                    className="px-4 py-2.5 rounded-xl bg-zinc-900 hover:bg-zinc-850 text-zinc-300 font-mono text-xs transition-colors cursor-pointer"
                  >
                    Cancel / Reset
                  </button>

                  <button
                    onClick={handleCommitImport}
                    disabled={!confirmGate || isCommitting}
                    className="flex items-center gap-2 py-2.5 px-6 rounded-xl bg-[#F59E0B] hover:bg-[#F59E0B]/90 disabled:opacity-40 text-black font-extrabold text-xs transition-all shadow-lg shadow-[#F59E0B]/20 cursor-pointer"
                  >
                    {isCommitting ? (
                      <>
                        <Activity className="h-4 w-4 animate-spin text-black" />
                        Executing Transactional Import...
                      </>
                    ) : (
                      <>
                        <Check className="h-4 w-4 stroke-[3]" />
                        Confirm &amp; Commit Import to Database
                      </>
                    )}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Import Result Banner (After Commit) */}
          {commitResult && (
            <div className="p-6 rounded-2xl bg-zinc-950 border border-[#F59E0B]/30 space-y-4 animate-in fade-in">
              <div className="flex items-center gap-3 text-[#F59E0B] font-bold text-base">
                <CheckCircle2 className="h-6 w-6 shrink-0" />
                <span>Import Completed Successfully!</span>
              </div>
              <p className="text-xs text-zinc-300 leading-relaxed">
                The master question bank was parsed and committed to PostgreSQL in an atomic transaction.
              </p>

              <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 pt-2">
                <div className="p-3 rounded-xl bg-black border border-zinc-850">
                  <span className="text-[10px] font-mono text-zinc-400 block">Created Questions</span>
                  <span className="text-lg font-bold font-mono text-[#F59E0B]">
                    {commitResult.created_questions}
                  </span>
                </div>
                <div className="p-3 rounded-xl bg-black border border-zinc-850">
                  <span className="text-[10px] font-mono text-zinc-400 block">Cross-Listed Links</span>
                  <span className="text-lg font-bold font-mono text-zinc-200">
                    {commitResult.cross_listed_rows}
                  </span>
                </div>
                <div className="p-3 rounded-xl bg-black border border-zinc-850">
                  <span className="text-[10px] font-mono text-zinc-400 block">Created Patterns</span>
                  <span className="text-lg font-bold font-mono text-[#F59E0B]">
                    {commitResult.created_patterns}
                  </span>
                </div>
                <div className="p-3 rounded-xl bg-black border border-zinc-850">
                  <span className="text-[10px] font-mono text-zinc-400 block">Unchanged Rows</span>
                  <span className="text-lg font-bold font-mono text-zinc-300">
                    {commitResult.unchanged_questions}
                  </span>
                </div>
              </div>

              <div className="pt-2 flex items-center justify-end gap-3">
                <button
                  onClick={() => {
                    setActiveTab("catalog");
                    fetchQuestions();
                  }}
                  className="flex items-center gap-2 px-4 py-2 rounded-xl bg-[#F59E0B] hover:bg-[#F59E0B]/90 text-black font-bold text-xs transition-all shadow-md shadow-[#F59E0B]/20 cursor-pointer"
                >
                  <Layers className="h-3.5 w-3.5" />
                  View Questions in Catalog
                </button>
              </div>
            </div>
          )}

          {/* Import History Table */}
          <div className="p-6 rounded-2xl bg-zinc-950 border border-zinc-900 space-y-4">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <Clock className="h-4 w-4 text-[#F59E0B]" />
                <h3 className="text-sm font-bold text-white">Import History &amp; Audit Log</h3>
              </div>
              <button
                onClick={fetchHistory}
                disabled={loadingHistory}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-zinc-900 hover:bg-zinc-850 text-zinc-300 text-xs font-mono transition-colors cursor-pointer"
              >
                <RefreshCw className={`h-3 w-3 ${loadingHistory ? "animate-spin text-[#F59E0B]" : ""}`} />
                Refresh
              </button>
            </div>

            <div className="rounded-xl border border-zinc-900 overflow-hidden bg-black">
              <table className="w-full text-left border-collapse text-xs">
                <thead>
                  <tr className="border-b border-zinc-900 bg-zinc-950 text-[10px] font-mono uppercase text-zinc-500">
                    <th className="py-2.5 px-3">Timestamp</th>
                    <th className="py-2.5 px-3">Filename / Source</th>
                    <th className="py-2.5 px-3 w-20">Total</th>
                    <th className="py-2.5 px-3 w-20">Valid</th>
                    <th className="py-2.5 px-3 w-20">Created</th>
                    <th className="py-2.5 px-3 w-20">Updated</th>
                    <th className="py-2.5 px-3 w-24">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-900 text-[11px] font-mono">
                  {loadingHistory ? (
                    <tr>
                      <td colSpan={7} className="py-6 text-center text-zinc-500">
                        Loading history...
                      </td>
                    </tr>
                  ) : importHistory.length === 0 ? (
                    <tr>
                      <td colSpan={7} className="py-6 text-center text-zinc-500">
                        No imports recorded yet.
                      </td>
                    </tr>
                  ) : (
                    importHistory.map((h) => (
                      <tr key={h.id} className="hover:bg-zinc-900/40">
                        <td className="py-2.5 px-3 text-zinc-400">
                          {new Date(h.created_at).toLocaleString()}
                        </td>
                        <td className="py-2.5 px-3 text-zinc-200 font-semibold">{h.filename}</td>
                        <td className="py-2.5 px-3 text-zinc-300">{h.total_rows}</td>
                        <td className="py-2.5 px-3 text-[#F59E0B]">{h.valid_rows}</td>
                        <td className="py-2.5 px-3 text-[#F59E0B]">{h.created_questions}</td>
                        <td className="py-2.5 px-3 text-zinc-300">{h.updated_questions}</td>
                        <td className="py-2.5 px-3">
                          <span
                            className={`px-1.5 py-0.5 rounded text-[9px] font-bold border ${
                              h.status === "COMPLETED"
                                ? "bg-[#F59E0B]/15 text-[#F59E0B] border-[#F59E0B]/30"
                                : "bg-zinc-900 text-zinc-400 border-zinc-800"
                            }`}
                          >
                            {h.status}
                          </span>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 3: PROMOTIONAL CREDITS GRANT VIEW                                     */}
      {/* ========================================================================= */}
      {activeTab === "grants" && (
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          <div className="lg:col-span-7 space-y-6">
            <div className="p-6 rounded-2xl bg-zinc-950 border border-zinc-900 space-y-5">
              <div>
                <h2 className="text-base font-bold text-white flex items-center gap-2">
                  <Coins className="h-5 w-5 text-[#F59E0B]" />
                  Administrative Promotional Grant
                </h2>
                <p className="text-xs text-zinc-400 mt-1">
                  Inject promotional or compensatory credits directly into a candidate&apos;s ledger.
                  Creates an atomic <code className="text-[#F59E0B]">ADMIN_ADJUSTMENT</code> entry.
                </p>
              </div>

              {grantSuccessMsg && (
                <div className="p-4 rounded-xl bg-zinc-950 border border-[#F59E0B]/30 text-zinc-200 text-xs flex items-start gap-2.5">
                  <CheckCircle2 className="h-4 w-4 text-[#F59E0B] shrink-0 mt-0.5" />
                  <span>{grantSuccessMsg}</span>
                </div>
              )}

              {grantErrorMsg && (
                <div className="p-4 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-300 text-xs flex items-start gap-2.5">
                  <AlertCircle className="h-4 w-4 text-[#F59E0B] shrink-0 mt-0.5" />
                  <span>{grantErrorMsg}</span>
                </div>
              )}

              <form onSubmit={handleGrant} className="space-y-4">
                <div>
                  <label className="block text-xs font-medium text-zinc-300 mb-1.5 uppercase font-mono">
                    Target User UUID
                  </label>
                  <input
                    type="text"
                    required
                    value={targetUserId}
                    onChange={(e) => setTargetUserId(e.target.value)}
                    placeholder="e.g. 550e8400-e29b-41d4-a716-446655440000"
                    className="w-full px-3.5 py-2.5 rounded-xl bg-black border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:outline-none focus:border-[#F59E0B] text-xs font-mono transition-colors"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-zinc-300 mb-1.5 uppercase font-mono">
                    Credit Amount
                  </label>
                  <input
                    type="number"
                    required
                    min={1}
                    max={10000}
                    value={grantAmount}
                    onChange={(e) => setGrantAmount(Number(e.target.value))}
                    className="w-full px-3.5 py-2.5 rounded-xl bg-black border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:outline-none focus:border-[#F59E0B] text-xs font-mono transition-colors"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-zinc-300 mb-1.5 uppercase font-mono">
                    Grant Reason / Justification
                  </label>
                  <textarea
                    required
                    value={grantReason}
                    onChange={(e) => setGrantReason(e.target.value)}
                    placeholder="e.g. Beta testing feedback stipend / customer compensation"
                    rows={2}
                    className="w-full p-3 rounded-xl bg-black border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:outline-none focus:border-[#F59E0B] text-xs transition-colors"
                  />
                </div>

                <div className="pt-2">
                  <button
                    type="submit"
                    disabled={isSubmittingGrant}
                    className="w-full flex items-center justify-center gap-2 py-3 px-4 rounded-xl bg-[#F59E0B] hover:bg-[#F59E0B]/90 text-black font-bold text-xs transition-all shadow-md shadow-[#F59E0B]/20 disabled:opacity-50 cursor-pointer"
                  >
                    <Send className="h-3.5 w-3.5" />
                    {isSubmittingGrant ? "Executing Transaction..." : "Grant Promotional Credits"}
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 4: SYSTEM INVARIANTS & DIAGNOSTICS VIEW                               */}
      {/* ========================================================================= */}
      {activeTab === "system" && (
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
          <div className="lg:col-span-7 space-y-4">
            <div className="p-6 rounded-2xl bg-zinc-950 border border-zinc-900 space-y-4">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <Database className="h-4 w-4 text-[#F59E0B]" />
                Runtime Architecture Invariants
              </h3>

              <div className="space-y-3 text-xs">
                <div className="p-3 rounded-xl bg-black border border-zinc-850 flex items-center justify-between">
                  <span className="text-zinc-400">PostgreSQL Migrations:</span>
                  <span className="font-mono text-[#F59E0B]">00008 Goose Applied (Excel Order &amp; Roadmaps)</span>
                </div>

                <div className="p-3 rounded-xl bg-black border border-zinc-850 flex items-center justify-between">
                  <span className="text-zinc-400">Catalog Archival Mode:</span>
                  <span className="font-mono text-zinc-200">Soft Deactivate (is_active boolean)</span>
                </div>

                <div className="p-3 rounded-xl bg-black border border-zinc-850 flex items-center justify-between">
                  <span className="text-zinc-400">AI Model Identifier:</span>
                  <span className="font-mono text-zinc-200">gemini-1.5-flash</span>
                </div>

                <div className="p-3 rounded-xl bg-black border border-zinc-850 flex items-center justify-between">
                  <span className="text-zinc-400">Ledger Concurrency:</span>
                  <span className="font-mono text-zinc-200">SELECT ... FOR UPDATE</span>
                </div>

                <div className="p-3 rounded-xl bg-black border border-zinc-850 flex items-center justify-between">
                  <span className="text-zinc-400">Session Security:</span>
                  <span className="font-mono text-zinc-300">CSRF Token + HttpOnly Cookie</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* ADD / EDIT QUESTION MODAL                                                 */}
      {/* ========================================================================= */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in">
          <div className="w-full max-w-xl rounded-2xl bg-zinc-950 border border-zinc-900 shadow-2xl p-6 space-y-5">
            <div className="flex items-center justify-between border-b border-zinc-900 pb-3">
              <h3 className="text-base font-bold text-white flex items-center gap-2">
                {editingQuestion ? (
                  <>
                    <Edit2 className="h-4 w-4 text-[#F59E0B]" />
                    Edit Question Details
                  </>
                ) : (
                  <>
                    <Plus className="h-4 w-4 text-[#F59E0B]" />
                    Add New Algorithmic Question
                  </>
                )}
              </h3>
              <button
                onClick={() => setIsModalOpen(false)}
                className="text-zinc-400 hover:text-white p-1 rounded-lg hover:bg-zinc-900 transition-colors cursor-pointer"
              >
                <X className="h-4 w-4" />
              </button>
            </div>

            {modalError && (
              <div className="p-3 rounded-xl bg-zinc-950 border border-zinc-800 text-zinc-300 text-xs flex items-center gap-2">
                <AlertCircle className="h-4 w-4 text-[#F59E0B] shrink-0" />
                <span>{modalError}</span>
              </div>
            )}

            <form onSubmit={handleSaveQuestion} className="space-y-4">
              {/* Title */}
              <div>
                <label className="block text-xs font-mono text-zinc-300 mb-1">
                  Problem Title <span className="text-[#F59E0B]">*</span>
                </label>
                <input
                  type="text"
                  required
                  value={formTitle}
                  onChange={(e) => setFormTitle(e.target.value)}
                  placeholder="e.g. Two Sum"
                  className="w-full px-3 py-2 rounded-xl bg-black border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:outline-none focus:border-[#F59E0B] text-xs"
                />
              </div>

              {/* Difficulty & LeetCode ID */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-mono text-zinc-300 mb-1">Difficulty</label>
                  <select
                    value={formDifficulty}
                    onChange={(e) => setFormDifficulty(e.target.value as Difficulty)}
                    className="w-full px-3 py-2 rounded-xl bg-black border border-zinc-800 text-zinc-100 focus:outline-none focus:border-[#F59E0B] text-xs"
                  >
                    <option value="EASY">Easy</option>
                    <option value="MEDIUM">Medium</option>
                    <option value="HARD">Hard</option>
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-mono text-zinc-300 mb-1">LeetCode ID</label>
                  <input
                    type="text"
                    value={formLeetCodeId}
                    onChange={(e) => setFormLeetCodeId(e.target.value)}
                    placeholder="e.g. 1"
                    className="w-full px-3 py-2 rounded-xl bg-black border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:outline-none focus:border-[#F59E0B] text-xs font-mono"
                  />
                </div>
              </div>

              {/* Primary Pattern */}
              <div>
                <label className="block text-xs font-mono text-zinc-300 mb-1">
                  Primary Pattern
                </label>
                <select
                  value={formPatternId}
                  onChange={(e) => setFormPatternId(e.target.value)}
                  className="w-full px-3 py-2 rounded-xl bg-black border border-zinc-800 text-zinc-100 focus:outline-none focus:border-[#F59E0B] text-xs"
                >
                  <option value="">None / Unassigned</option>
                  {patterns.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </div>

              {/* Core Checkbox & Time Limit */}
              <div className="grid grid-cols-2 gap-4 items-center">
                <label className="flex items-center gap-2 cursor-pointer pt-2">
                  <input
                    type="checkbox"
                    checked={formIsCore}
                    onChange={(e) => setFormIsCore(e.target.checked)}
                    className="h-4 w-4 rounded bg-black border-zinc-700 text-[#F59E0B] focus:ring-[#F59E0B]/20"
                  />
                  <span className="text-xs text-zinc-300 font-mono">Core Blind 75 Problem</span>
                </label>

                <div>
                  <label className="block text-xs font-mono text-zinc-300 mb-1">
                    Time Limit (Mins)
                  </label>
                  <input
                    type="number"
                    min={5}
                    max={180}
                    value={formTimeLimit}
                    onChange={(e) => setFormTimeLimit(Number(e.target.value))}
                    className="w-full px-3 py-2 rounded-xl bg-black border border-zinc-800 text-zinc-100 focus:outline-none focus:border-[#F59E0B] text-xs font-mono"
                  />
                </div>
              </div>

              {/* Editorial Summary */}
              <div>
                <label className="block text-xs font-mono text-zinc-300 mb-1">
                  Editorial Guidance / Notes
                </label>
                <textarea
                  rows={3}
                  value={formEditorial}
                  onChange={(e) => setFormEditorial(e.target.value)}
                  placeholder="Key algorithmic insights, corner cases, or constraints..."
                  className="w-full p-2.5 rounded-xl bg-black border border-zinc-800 text-zinc-100 placeholder:text-zinc-600 focus:outline-none focus:border-[#F59E0B] text-xs"
                />
              </div>

              {/* Action Buttons */}
              <div className="pt-3 border-t border-zinc-900 flex items-center justify-end gap-3">
                <button
                  type="button"
                  onClick={() => setIsModalOpen(false)}
                  className="px-4 py-2 rounded-xl bg-zinc-900 hover:bg-zinc-850 text-zinc-300 text-xs font-mono transition-colors cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={isSavingQuestion}
                  className="px-5 py-2 rounded-xl bg-[#F59E0B] hover:bg-[#F59E0B]/90 text-black font-bold text-xs transition-all shadow-md shadow-[#F59E0B]/20 disabled:opacity-50 cursor-pointer"
                >
                  {isSavingQuestion ? "Saving..." : editingQuestion ? "Update Question" : "Create Question"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
