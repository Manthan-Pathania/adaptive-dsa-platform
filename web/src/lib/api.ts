import {
  AIPricingResponse,
  BalanceResponse,
  DailyPlan,
  ExplainResult,
  HintResult,
  LedgerEntry,
  Pattern,
  PatternMetric,
  Question,
  QuestionAttempt,
  RevisionSchedule,
  ReviewResult,
  User,
  UserQuestionProgress,
  CreditPackage,
  PaymentOrder,
  CreateOrderResult,
  VerifyCheckoutRequest,
  VerifyCheckoutResponse,
  ImportPreviewResult,
  ImportResult,
  ImportHistory,
  CreateQuestionPayload,
  UpdateQuestionPayload,
  UserProfile,
  Roadmap,
  PatternProgressItem,
  GenerateRoadmapPayload,
  QuickMarkPayload,
} from "@/types";

const API_BASE = "/api/v1";

let csrfToken: string | null = null;

export function setCSRFToken(token: string) {
  csrfToken = token;
}

export function getCSRFToken(): string | null {
  return csrfToken;
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers || {});
  if (!(options.body instanceof FormData)) {
    headers.set("Content-Type", "application/json");
  }

  if (csrfToken && options.method && ["POST", "PUT", "DELETE", "PATCH"].includes(options.method.toUpperCase())) {
    headers.set("X-CSRF-Token", csrfToken);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
    credentials: "include",
  });

  const headerCsrf = response.headers.get("X-CSRF-Token");
  if (headerCsrf) {
    csrfToken = headerCsrf;
  }

  if (!response.ok) {
    let errorDetail = "An unexpected error occurred";
    let errorCode = "UNKNOWN_ERROR";
    try {
      const errJson = await response.json();
      errorDetail = errJson.detail || errJson.title || errorDetail;
      errorCode = errJson.code || errorCode;
    } catch {
      errorDetail = response.statusText || errorDetail;
    }

    const err = new Error(errorDetail) as Error & { status: number; code: string };
    err.status = response.status;
    err.code = errorCode;
    throw err;
  }

  if (response.status === 204) {
    return {} as T;
  }

  return response.json();
}

export const api = {
  // Auth
  async register(email: string, password: string, fullName: string): Promise<{ user: User; csrf_token: string }> {
    const data = await request<{ user: User; csrf_token: string }>("/auth/register", {
      method: "POST",
      body: JSON.stringify({ email, password, full_name: fullName }),
    });
    if (data.csrf_token) {
      setCSRFToken(data.csrf_token);
    }
    return data;
  },

  async login(email: string, password: string): Promise<{ user: User; csrf_token: string }> {
    const data = await request<{ user: User; csrf_token: string }>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ email, password }),
    });
    if (data.csrf_token) {
      setCSRFToken(data.csrf_token);
    }
    return data;
  },

  async logout(): Promise<void> {
    await request<void>("/auth/logout", { method: "POST" });
    csrfToken = null;
  },

  async me(): Promise<{ user: User; csrf_token?: string }> {
    const data = await request<{ user: User; csrf_token?: string }>("/auth/me");
    if (data.csrf_token) {
      setCSRFToken(data.csrf_token);
    }
    return data;
  },

  // User Profile & Onboarding
  async getProfile(): Promise<UserProfile> {
    return request<UserProfile>("/auth/profile");
  },

  async updateProfile(data: Partial<UserProfile>): Promise<UserProfile> {
    return request<UserProfile>("/auth/profile", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  // Roadmaps
  async listRoadmaps(): Promise<{ roadmaps: Roadmap[] }> {
    return request<{ roadmaps: Roadmap[] }>("/roadmaps");
  },

  async getRoadmap(idOrSlug: string): Promise<Roadmap> {
    return request<Roadmap>(`/roadmaps/${encodeURIComponent(idOrSlug)}`);
  },

  async importRoadmap(formData: FormData): Promise<Roadmap> {
    return request<Roadmap>("/roadmaps/import", {
      method: "POST",
      body: formData,
    });
  },

  async downloadRoadmapTemplate(): Promise<Blob> {
    const response = await fetch(`${API_BASE}/roadmaps/template`, {
      credentials: "include",
    });
    if (!response.ok) {
      throw new Error("Failed to download roadmap template");
    }
    return response.blob();
  },

  async deleteRoadmap(id: string): Promise<void> {
    return request<void>(`/roadmaps/${encodeURIComponent(id)}`, {
      method: "DELETE",
    });
  },

  // Adaptive Preparation
  async getDailyPlan(roadmapId?: string): Promise<DailyPlan> {
    const query = roadmapId ? `?roadmap_id=${encodeURIComponent(roadmapId)}` : "";
    return request<DailyPlan>(`/adaptive/daily${query}`);
  },

  async getWeakPatterns(): Promise<{ weak_patterns: PatternMetric[]; count: number }> {
    return request<{ weak_patterns: PatternMetric[]; count: number }>("/adaptive/weak-patterns");
  },

  // Question Catalog
  async listQuestions(params?: {
    search?: string;
    difficulty?: string;
    topic_id?: string;
    pattern_id?: string;
    limit?: number;
    offset?: number;
  }): Promise<{ questions: Question[]; total: number }> {
    const q = new URLSearchParams();
    if (params?.search) q.set("search", params.search);
    if (params?.difficulty) q.set("difficulty", params.difficulty);
    if (params?.topic_id) q.set("topic_id", params.topic_id);
    if (params?.pattern_id) q.set("pattern_id", params.pattern_id);
    if (params?.limit) q.set("limit", params.limit.toString());
    if (params?.offset) q.set("offset", params.offset.toString());
    const query = q.toString() ? `?${q.toString()}` : "";
    return request<{ questions: Question[]; total: number }>(`/questions${query}`);
  },

  async getQuestion(idOrSlug: string): Promise<Question> {
    return request<Question>(`/questions/${encodeURIComponent(idOrSlug)}`);
  },

  // Progress & Spaced Repetition
  async recordAttempt(
    questionId: string,
    payload: {
      outcome: string;
      time_spent_secs: number;
      difficulty_experienced: string;
      quality_rating: number;
      mistake_tags: string[];
      approach_summary?: string;
      code_snippet?: string;
    }
  ): Promise<{ attempt: QuestionAttempt; progress: UserQuestionProgress; revision?: RevisionSchedule }> {
    return request<{ attempt: QuestionAttempt; progress: UserQuestionProgress; revision?: RevisionSchedule }>(
      `/questions/${encodeURIComponent(questionId)}/attempts`,
      {
        method: "POST",
        body: JSON.stringify(payload),
      }
    );
  },

  async getQuestionProgress(questionId: string): Promise<{ progress: UserQuestionProgress; attempts: QuestionAttempt[] }> {
    return request<{ progress: UserQuestionProgress; attempts: QuestionAttempt[] }>(
      `/questions/${encodeURIComponent(questionId)}/progress`
    );
  },

  async getProgressSummary(): Promise<{
    stats: { total_solved: number; total_mastered: number; total_in_flight: number };
    recent_attempts: QuestionAttempt[];
  }> {
    return request<{
      stats: { total_solved: number; total_mastered: number; total_in_flight: number };
      recent_attempts: QuestionAttempt[];
    }>("/progress");
  },

  async listAllUserProgress(): Promise<{ progress: UserQuestionProgress[] }> {
    return request<{ progress: UserQuestionProgress[] }>("/progress/all");
  },

  async quickMarkProgress(payload: QuickMarkPayload): Promise<{ status: string; progress: UserQuestionProgress }> {
    return request<{ status: string; progress: UserQuestionProgress }>("/progress/quick-mark", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  async unmarkProgress(questionId: string): Promise<{ status: string }> {
    return request<{ status: string }>("/progress/unmark", {
      method: "POST",
      body: JSON.stringify({ question_id: questionId }),
    });
  },

  async getPatternsProgress(): Promise<{ patterns: PatternProgressItem[] }> {
    return request<{ patterns: PatternProgressItem[] }>("/catalog/patterns-progress");
  },

  async generateRoadmap(payload: GenerateRoadmapPayload): Promise<Roadmap> {
    return request<Roadmap>("/roadmaps/generate", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  async listDueRevisions(limit = 20, offset = 0): Promise<{ revisions: RevisionSchedule[]; total: number }> {
    return request<{ revisions: RevisionSchedule[]; total: number }>(`/revisions/due?limit=${limit}&offset=${offset}`);
  },

  async exportUserData(): Promise<void> {
    const res = await fetch("/api/v1/user/export", {
      credentials: "include",
    });
    if (!res.ok) {
      throw new Error("Failed to export learning data");
    }
    const blob = await res.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `adaptive_dsa_learning_export_${new Date().toISOString().split("T")[0]}.json`;
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(a);
  },


  // AI Tutoring & Progressive Hints
  async requestHint(questionId: string, level: number, candidateNotes: string): Promise<HintResult> {
    return request<HintResult>("/ai/hints", {
      method: "POST",
      body: JSON.stringify({ question_id: questionId, level, candidate_notes: candidateNotes }),
    });
  },

  async reviewApproach(
    questionId: string,
    candidateCode: string,
    proposedTime = "O(N)",
    proposedSpace = "O(1)"
  ): Promise<ReviewResult> {
    return request<ReviewResult>("/ai/reviews", {
      method: "POST",
      body: JSON.stringify({
        question_id: questionId,
        candidate_code: candidateCode,
        proposed_time: proposedTime,
        proposed_space: proposedSpace,
      }),
    });
  },

  async explainConcept(topicName: string, patternName: string, candidateQuery: string): Promise<ExplainResult> {
    return request<ExplainResult>("/ai/explain", {
      method: "POST",
      body: JSON.stringify({
        topic_name: topicName,
        pattern_name: patternName,
        candidate_query: candidateQuery,
      }),
    });
  },

  async getAIPricing(): Promise<AIPricingResponse> {
    return request<AIPricingResponse>("/ai/pricing");
  },

  // Credits & Billing
  async getCreditBalance(): Promise<BalanceResponse> {
    return request<BalanceResponse>("/credits/balance");
  },

  async getCreditLedger(limit = 20, offset = 0): Promise<{ entries: LedgerEntry[]; total: number }> {
    return request<{ entries: LedgerEntry[]; total: number }>(`/credits/ledger?limit=${limit}&offset=${offset}`);
  },

  async adminGrantCredits(targetUserId: string, amount: number, reason: string): Promise<{ entry: LedgerEntry }> {
    return request<{ entry: LedgerEntry }>(`/admin/users/${encodeURIComponent(targetUserId)}/credits`, {
      method: "POST",
      body: JSON.stringify({ amount, reason }),
    });
  },

  // Pattern Catalog
  async listPatterns(): Promise<{ patterns: Pattern[] }> {
    return request<{ patterns: Pattern[] }>("/patterns");
  },

  // Admin Question Bank Management
  async adminListQuestions(params?: {
    search?: string;
    difficulty?: string;
    pattern_id?: string;
    is_active?: string; // "true", "false", "all"
    limit?: number;
    offset?: number;
  }): Promise<{ questions: Question[]; total: number }> {
    const q = new URLSearchParams();
    if (params?.search) q.set("search", params.search);
    if (params?.difficulty) q.set("difficulty", params.difficulty);
    if (params?.pattern_id) q.set("pattern_id", params.pattern_id);
    if (params?.is_active) q.set("is_active", params.is_active);
    if (params?.limit) q.set("limit", params.limit.toString());
    if (params?.offset) q.set("offset", params.offset.toString());
    const query = q.toString() ? `?${q.toString()}` : "";
    return request<{ questions: Question[]; total: number }>(`/admin/questions${query}`);
  },

  async adminCreateQuestion(data: CreateQuestionPayload): Promise<Question> {
    return request<Question>("/admin/questions", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async adminUpdateQuestion(id: string, data: UpdateQuestionPayload): Promise<Question> {
    return request<Question>(`/admin/questions/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(data),
    });
  },

  async adminToggleArchive(id: string, isActive?: boolean): Promise<Question> {
    const body = isActive !== undefined ? JSON.stringify({ is_active: isActive }) : undefined;
    return request<Question>(`/admin/questions/${encodeURIComponent(id)}/archive`, {
      method: "PATCH",
      body,
    });
  },

  async adminImportPreview(fileOrPath: File | string): Promise<ImportPreviewResult> {
    let body: FormData | string;
    if (fileOrPath instanceof File) {
      const formData = new FormData();
      formData.append("file", fileOrPath);
      body = formData;
    } else {
      body = JSON.stringify({ file_path: fileOrPath });
    }

    return request<ImportPreviewResult>("/admin/questions/import/preview", {
      method: "POST",
      body,
    });
  },

  async adminImportCommit(fileOrPath: File | string): Promise<ImportResult> {
    let body: FormData | string;
    if (fileOrPath instanceof File) {
      const formData = new FormData();
      formData.append("file", fileOrPath);
      body = formData;
    } else {
      body = JSON.stringify({ file_path: fileOrPath });
    }

    return request<ImportResult>("/admin/questions/import/commit", {
      method: "POST",
      body,
    });
  },

  async adminListImportHistory(limit = 20, offset = 0): Promise<{ history: ImportHistory[] }> {
    return request<{ history: ImportHistory[] }>(`/admin/questions/import/history?limit=${limit}&offset=${offset}`);
  },

  // Payments & Credit Packages (Razorpay Test Mode)
  async getPaymentPackages(): Promise<{ packages: CreditPackage[] }> {
    return request<{ packages: CreditPackage[] }>("/payments/packages");
  },

  async createPaymentOrder(packageId: string, idempotencyKey?: string): Promise<CreateOrderResult> {
    return request<CreateOrderResult>("/payments/orders", {
      method: "POST",
      body: JSON.stringify({
        package_id: packageId,
        idempotency_key: idempotencyKey,
      }),
    });
  },

  async verifyPayment(data: VerifyCheckoutRequest): Promise<VerifyCheckoutResponse> {
    return request<VerifyCheckoutResponse>("/payments/verify", {
      method: "POST",
      body: JSON.stringify(data),
    });
  },

  async getPaymentHistory(limit = 20, offset = 0): Promise<{ orders: PaymentOrder[]; total: number }> {
    return request<{ orders: PaymentOrder[]; total: number }>(`/payments/history?limit=${limit}&offset=${offset}`);
  },
};
