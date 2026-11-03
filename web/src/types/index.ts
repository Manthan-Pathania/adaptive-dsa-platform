export type UserRole = "CANDIDATE" | "ADMIN";

export interface User {
  id: string;
  email: string;
  full_name: string;
  role: UserRole;
  is_active: boolean;
  created_at: string;
}

export type Difficulty = "EASY" | "MEDIUM" | "HARD";

export type ProgressStatus = "NOT_STARTED" | "IN_PROGRESS" | "SOLVED" | "MASTERED";

export type AttemptOutcome =
  | "SOLVED_INDEPENDENTLY"
  | "SOLVED_WITH_HINTS"
  | "SOLVED_AFTER_EDITORIAL"
  | "UNRESOLVED_FAILED";

export type PerceivedDifficulty = "EASY" | "JUST_RIGHT" | "CHALLENGING" | "OVERWHELMING";

export interface Pattern {
  id: string;
  topic_id?: string;
  name: string;
  slug: string;
  description?: string;
  topic_name?: string;
}

export interface Question {
  id: string;
  title: string;
  slug: string;
  external_url: string;
  difficulty: Difficulty;
  is_core: boolean;
  is_active?: boolean;
  leetcode_id?: string;
  editorial_summary?: string;
  sample_time_limit_mins?: number;
  excel_order?: number;
  patterns?: Pattern[];
  companies?: string[];
  created_at?: string;
  updated_at?: string;
}

export interface RowError {
  row: number;
  column: string;
  message: string;
}

export interface SamplePreviewRow {
  row_number: number;
  title: string;
  difficulty: Difficulty;
  pattern: string;
  subpattern?: string;
  leetcode_id?: string;
  external_url: string;
  action: "CREATE" | "UPDATE" | "UNCHANGED" | "CROSS_LIST";
}

export interface ImportPreviewResult {
  detected_sheet: string;
  total_rows_read: number;
  valid_rows: number;
  invalid_rows: number;
  unique_questions: number;
  cross_listed_rows: number;
  planned_creates: number;
  planned_updates: number;
  planned_unchanged: number;
  sample_rows: SamplePreviewRow[];
  row_errors?: RowError[];
  warnings?: string[];
}

export interface ImportResult {
  is_dry_run: boolean;
  total_rows_read: number;
  valid_rows: number;
  invalid_rows: number;
  created_questions: number;
  updated_questions: number;
  unchanged_questions: number;
  cross_listed_rows: number;
  created_topics: number;
  created_patterns: number;
  created_companies: number;
  row_errors?: RowError[];
  warnings?: string[];
}

export interface ImportHistory {
  id: string;
  filename: string;
  total_rows: number;
  valid_rows: number;
  created_questions: number;
  updated_questions: number;
  unchanged_questions: number;
  invalid_rows: number;
  status: string;
  imported_by?: string;
  created_at: string;
}

export interface CreateQuestionPayload {
  title: string;
  difficulty: Difficulty;
  is_core?: boolean;
  leetcode_id?: string;
  editorial_summary?: string;
  sample_time_limit_mins?: number;
  pattern_ids?: string[];
}

export interface UpdateQuestionPayload {
  title: string;
  difficulty: Difficulty;
  is_core?: boolean;
  leetcode_id?: string;
  editorial_summary?: string;
  sample_time_limit_mins?: number;
  pattern_ids?: string[];
}

export interface UserQuestionProgress {
  id: string;
  user_id: string;
  question_id: string;
  status: ProgressStatus;
  first_solved_at?: string;
  last_solved_at?: string;
  solve_count: number;
  independent_solve_count: number;
  total_time_spent_secs: number;
  is_bookmarked: boolean;
  personal_notes?: string;
}

export interface RevisionSchedule {
  id: string;
  user_id: string;
  question_id: string;
  stability: number;
  difficulty: number;
  repetition_count: number;
  ease_factor: number;
  interval_days: number;
  next_review_at: string;
  last_reviewed_at?: string;
  question_title: string;
  question_slug: string;
  question_difficulty: string;
}

export interface QuestionAttempt {
  id: string;
  user_id: string;
  question_id: string;
  outcome: AttemptOutcome;
  time_spent_secs: number;
  difficulty_experienced: PerceivedDifficulty;
  quality_rating: number;
  mistake_tags: string[];
  approach_summary?: string;
  code_snippet?: string;
  created_at: string;
  question_title?: string;
  question_slug?: string;
  question_difficulty?: string;
}

export type RecommendationCategory =
  | "DUE_REVISION"
  | "WEAK_PATTERN_REINFORCEMENT"
  | "ROADMAP_NEXT_STEP"
  | "CURATED_EXPLORATION";

export interface RecommendedQuestion {
  question_id: string;
  title: string;
  slug: string;
  difficulty: string;
  pattern_name?: string;
  category: RecommendationCategory;
  priority_score: number;
  rationale: string;
  days_overdue?: number;
}

export interface PatternMetric {
  pattern_id: string;
  pattern_name: string;
  pattern_slug: string;
  total_attempts: number;
  failed_attempts: number;
  independent_solves: number;
  hint_solves: number;
  avg_quality_rating: number;
  is_weak: boolean;
}

export interface DailyPlan {
  date: string;
  recommendations: RecommendedQuestion[];
  weak_patterns: PatternMetric[];
  due_revisions_count: number;
  overdue_revisions_count: number;
  daily_goal_met: boolean;
  generated_at: string;
}

export interface CreditAccount {
  user_id: string;
  cached_balance: number;
  updated_at: string;
}

export interface LedgerEntry {
  id: string;
  user_id: string;
  amount: number;
  entry_type: "PURCHASE" | "AI_USAGE" | "PROMOTIONAL_GRANT" | "REFUND" | "ADMIN_ADJUSTMENT";
  reference_type?: string;
  reference_id?: string;
  description: string;
  created_at: string;
}

export interface BalanceResponse {
  balance: number;
  recent_entries: LedgerEntry[];
}

export interface HintResult {
  level: number;
  hint_text: string;
  credits_deducted: number;
  remaining_balance: number;
  model_used: string;
}

export interface ReviewResult {
  review_text: string;
  credits_deducted: number;
  remaining_balance: number;
  model_used: string;
}

export interface ExplainResult {
  explanation: string;
  credits_deducted: number;
  remaining_balance: number;
  model_used: string;
}

export interface AIPricingResponse {
  hints: Record<string, number>;
  approach_review: number;
  concept_explain: number;
}

export const DEFAULT_AI_PRICING: AIPricingResponse = {
  hints: {
    "1": 2,
    "2": 2,
    "3": 4,
    "4": 4,
  },
  approach_review: 5,
  concept_explain: 3,
};

export interface CreditPackage {
  id: string;
  name: string;
  credits: number;
  amount_paise: number;
  currency: string;
  description: string;
  popular?: boolean;
}

export type PaymentOrderStatus = "CREATED" | "ATTEMPTED" | "PAID" | "FAILED";

export interface PaymentOrder {
  id: string;
  user_id: string;
  package_id: string;
  amount_paise: number;
  currency: string;
  credits_amount: number;
  status: PaymentOrderStatus;
  razorpay_order_id?: string;
  razorpay_payment_id?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateOrderResult {
  order_id: string;
  razorpay_order_id: string;
  amount_paise: number;
  currency: string;
  credits_amount: number;
  package_id: string;
  package_name: string;
  key_id: string;
}

export interface VerifyCheckoutRequest {
  razorpay_order_id: string;
  razorpay_payment_id: string;
  razorpay_signature: string;
}

export interface VerifyCheckoutResponse {
  status: string;
  message: string;
  order: PaymentOrder;
  credits_granted: number;
}

export interface UserProfile {
  user_id: string;
  target_companies: string[];
  target_date?: string;
  timeline_label: string;
  daily_study_mins: number;
  preferred_days: string[];
  experience_level: string;
  familiar_patterns: string[];
  daily_goal_questions: number;
  is_onboarding_completed: boolean;
  selected_roadmap_id?: string;
  diagnostic_results?: Record<string, unknown>;
  created_at?: string;
  updated_at?: string;
}

export interface RoadmapItem {
  id: string;
  roadmap_id: string;
  question_id: string;
  stage_number: number;
  order_index: number;
  is_mandatory: boolean;
  question_title: string;
  question_slug: string;
  difficulty: Difficulty;
  is_core: boolean;
  excel_order?: number;
  pattern_name?: string;
  subpattern_name?: string;
  progress_status?: ProgressStatus;
  solve_count?: number;
  last_solved_at?: string;
}

export interface Roadmap {
  id: string;
  user_id?: string;
  title: string;
  slug: string;
  description?: string;
  target_days?: number;
  is_system_template?: boolean;
  is_curated?: boolean;
  is_company_specific?: boolean;
  company_id?: string;
  company_name?: string;
  total_questions?: number;
  items?: RoadmapItem[];
}

export interface PatternProgressItem {
  pattern_id: string;
  pattern_name: string;
  pattern_slug: string;
  topic_name: string;
  total_questions: number;
  solved_count: number;
  percent: number;
}

export interface GenerateRoadmapPayload {
  target_companies: string[];
  timeline_days: number;
  daily_hours: number;
  experience_level: string;
  familiar_patterns?: string[];
  focus_notes?: string;
}

export interface QuickMarkPayload {
  question_id: string;
  outcome?: AttemptOutcome;
  difficulty_experienced?: PerceivedDifficulty;
  needs_revision?: boolean;
  confidence_first_try?: boolean;
  notes?: string;
}



