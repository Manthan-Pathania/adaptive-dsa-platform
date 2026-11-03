# Master Question Bank Import & Admin Catalog Management

## 1. Overview & Source of Truth

The authoritative master DSA question bank is sourced from the user's master workbook:

- **Host Path**: `F:\DSA\FAANG_DSA_Master_Plan_v2.xlsx`
- **Format**: Microsoft Excel OpenXML (`.xlsx`)
- **Primary Sheet**: `Master Plan`
- **Total Problem Rows**: 621
- **Curated Patterns**: 29 algorithmic patterns
- **Unique Questions**: 542 (539 imported + 3 initial seed)
- **Cross-Listed Associations**: 82 problem-pattern links (problems cross-listed across multiple patterns)

All questions are stored in PostgreSQL (`questions`, `patterns`, `topics`, `question_patterns`, `question_import_history`). Questions are **never hardcoded** into application source code.

---

## 2. Workbook Structure & Database Mapping

### Column Mapping Matrix

| Workbook Column Header | Inferred Field | PostgreSQL Target | Description / Transform |
| :--- | :--- | :--- | :--- |
| `Problem` | Title | `questions.title` | Full title of the algorithmic problem. |
| *(Synthesized)* | Slug | `questions.slug` | URL-safe lowercased slug (`two-sum`). |
| *(Synthesized)* | External URL | `questions.external_url` | Canonical URL: `https://leetcode.com/problems/<slug>/`. Unique index key for idempotency. |
| `Difficulty` | Difficulty | `questions.difficulty` | Enum: `EASY`, `MEDIUM`, `HARD` (case-insensitive parsed). |
| `LeetCode ID` | External ID | `questions.leetcode_id` | Alphanumeric LeetCode identifier (e.g. `1`, `283`, `GFG`). |
| `Pattern` | Primary Pattern | `patterns.name` & `question_patterns` | Algorithmic pattern name. Linked with `is_primary = TRUE` on first occurrence. |
| `Pattern` | Topic Name | `topics.name` | Pattern category name in the curriculum. |
| `Subpattern` | Subpattern | `patterns.name` & `question_patterns` | Fine-grained classification (linked with `is_primary = FALSE`). |
| `Notes` | Editorial Guidance | `questions.editorial_summary` | Algorithmic summary or corner-case guidance. |
| *(Migration 00006)* | Active Status | `questions.is_active` | Boolean flag. `TRUE` = visible to candidates; `FALSE` = archived. Default `TRUE`. |

### Handling Cross-Listed Questions
79 questions in the master workbook appear in multiple rows because they belong to multiple algorithmic patterns (e.g., *Move Zeroes* appears under both *Arrays & Hashing* and *Two Pointers*).
- **Rule**: When a question's `external_url` has already been encountered in the import, subsequent occurrences link the additional pattern in `question_patterns` (`is_primary = FALSE`) rather than rejecting the row or duplicating the question.

---

## 3. Database Schema Changes (Migration 00006)

Applied via Goose migration `00006_catalog_admin_and_archival.sql`:

1. **`questions.is_active`** (`BOOLEAN NOT NULL DEFAULT TRUE`):
   - Supports safe soft-deactivation. Candidates querying `/api/v1/questions` only see `is_active = TRUE`.
   - Candidate attempts (`question_attempts`), solved status (`user_question_progress`), and spaced repetition schedules (`spaced_repetition_items`) are **permanently preserved**.
2. **`questions.leetcode_id`** (`VARCHAR(50)`):
   - Stores the original LeetCode / platform identifier.
3. **`question_import_history`**:
   - Audit trail table tracking every import execution: `id`, `filename`, `total_rows`, `valid_rows`, `created_questions`, `updated_questions`, `unchanged_questions`, `invalid_rows`, `status`, `imported_by`, `created_at`.

---

## 4. Admin Question Bank Management Workflows

### A. Bulk Excel Import via Admin UI (`/admin`)

The platform supports ongoing question management through the web interface without requiring command-line or SQL access:

1. **Upload Selection**:
   - Navigate to `http://localhost:3000/admin` $\rightarrow$ switch to the **Bulk Excel Import** tab.
   - Choose **Browser Upload** (drag & drop or select `F:\DSA\FAANG_DSA_Master_Plan_v2.xlsx` from Windows Explorer) or **Host File Path**.
2. **Preview & Validation Gate**:
   - Click **Preview & Validate Workbook**.
   - The backend validates all rows against schema rules, checks existing database records, and generates an interactive **Validation Scorecard**:
     - Detected Sheet: `Master Plan`
     - Total Rows Read: `621`
     - Valid Rows: `621`
     - Invalid Rows: `0`
     - Unique Questions: `539`
     - Cross-Listed Links: `82`
     - Planned Creates: `539` (or `0` on re-import)
     - Planned Updates / Unchanged: `531`
     - First 15 Sample Rows Preview with proposed action badges (`CREATE`, `CROSS_LIST`, `UNCHANGED`).
3. **Explicit Confirmation Gate**:
   - Check the confirmation box: *"I have reviewed the validation scorecard and confirm committing these changes to PostgreSQL."*
   - Click **Confirm & Commit Import to Database**.
   - The transaction executes atomically, logs an audit record in `question_import_history`, and displays the final import metrics banner.

### B. Question Bank CRUD & Archival (`/admin`)

Under the **Question Bank Catalog** tab:

1. **Search & Filter**:
   - Real-time search by problem title or LeetCode ID.
   - Filter dropdowns for Difficulty (`Easy`, `Medium`, `Hard`), Pattern, and Catalog Status (`Active Only`, `Archived Only`, `All Statuses`).
2. **Add Question**:
   - Click **Add Question** to open the modal.
   - Enter Problem Title, Difficulty, Primary Pattern, LeetCode ID, Core flag, and Editorial Guidance.
   - Saves via `POST /api/v1/admin/questions`.
3. **Edit Question**:
   - Click the pencil icon on any row to open the edit modal.
   - Updates title, difficulty, pattern, or notes via `PUT /api/v1/admin/questions/{id}`.
4. **Archive / Restore (Soft Deactivation)**:
   - Click the archive icon on any active problem: instantly toggles `is_active = false`.
   - The question is removed from the active candidate catalog (`/questions`), while all historical submissions, code revisions, and stats remain intact.
   - Click the restore icon to reactivate the question in the candidate catalog.

---

## 5. CLI Importer (Operational Fallback)

The CLI tool remains fully functional for scriptable or automated operational pipelines:

```powershell
# Dry-run validation (zero database writes)
go run ./cmd/import-questions --file="F:\DSA\FAANG_DSA_Master_Plan_v2.xlsx" --dry-run

# Atomic commit transaction
go run ./cmd/import-questions --file="F:\DSA\FAANG_DSA_Master_Plan_v2.xlsx" --commit
```

---

## 6. Local Testing & Verification Procedure

Follow these beginner-friendly steps to test the end-to-end import locally:

1. **Ensure Local Stack is Running**:
   ```powershell
   docker compose -f docker-compose.local.yml up -d
   ```
2. **Sign In as Administrator**:
   - Open browser: `http://localhost:3000/login`
   - Sign in with the provisioned admin account:
     - Email: `admin@adaptivedsa.local`
     - Password: `AdminPassword123!`
     *(Alternatively, sign in with `manthanpathania03@gmail.com` if you registered locally; it has also been granted `ADMIN` role).*
3. **Open Admin Command Center**:
   - Navigate to `http://localhost:3000/admin`.
   - The role badge will display `SUPER_ADMIN`.
4. **Preview Workbook Import**:
   - Switch to the **Bulk Excel Import** tab.
   - Click the upload box and select `F:\DSA\FAANG_DSA_Master_Plan_v2.xlsx`.
   - Click **Preview & Validate Workbook**.
   - Verify the scorecard shows `621 Total Rows`, `621 Valid Rows`, `0 Invalid Rows`.
5. **Commit Import**:
   - Check the confirmation checkbox and click **Confirm & Commit Import to Database**.
   - Verify the success banner reports questions created and patterns linked.
6. **Verify Candidate Catalog**:
   - Navigate to `http://localhost:3000/questions`.
   - Browse through the 539+ problems, search for "3Sum" or "Two Pointers", and verify pattern tags and difficulty badges render correctly.
7. **Test Idempotency**:
   - Return to `http://localhost:3000/admin` $\rightarrow$ **Bulk Excel Import**.
   - Run **Preview & Validate Workbook** on the same file again.
   - Notice: **Planned Creates** is `0` and sample rows show `UNCHANGED`. Zero duplicate questions are created.
