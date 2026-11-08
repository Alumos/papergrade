export type User = {
  id: string;
  username: string;
};
export type Session = {
  user: User;
  storage_mode: string;
  capabilities?: Record<string, unknown>;
};
export type Class = {
  id: string;
  name: string;
  student_count: number;
  created_at: string;
};
export type Student = {
  id: string;
  class_id: string;
  number: string;
  name: string;
};
export type Question = {
  id?: string;
  number: string;
  section?: string;
  type: "single_choice" | "multiple_choice" | "true_false";
  options: string[];
  answer: string[];
  score_units: number;
  partial_score_units?: number;
  review_flags?: string[];
};
export type Exam = {
  id: string;
  title: string;
  subject: string;
  class_id: string;
  status: "draft" | "published";
  revision: number;
  grading_version?: string;
  questions: Question[];
  release_id?: string;
  created_at: string;
  student_count: number;
  student_number_digits?: number;
  received_count: number;
  review_count: number;
};
export type Sheet = {
  student_id: string;
  student_number: string;
  student_name: string;
  token: string;
};
export type RecognizedAnswer = {
  question_id: string;
  selected: string[];
  state: string;
  confidence?: number;
};
export type IdentityState =
  "recognized" | "blank" | "uncertain" | "invalid_multiple" | "not_required";
export type CaptureStatus =
  | "queued"
  | "processing"
  | "committed"
  | "needs_review"
  | "duplicate"
  | "conflict"
  | "failed";
export type CaptureResult = {
  capture_id: string;
  client_capture_id: string;
  status: CaptureStatus;
  student_display?: string;
  student_number?: string;
  identity_state?: IdentityState;
  revision?: number;
  submission_id?: string;
  score_units?: number;
  max_score_units?: number;
  review_count?: number;
  message?: string;
  deduplicated?: boolean;
  answers?: RecognizedAnswer[];
};
export type Result = {
  id: string;
  student_id: string;
  student_number: string;
  student_name: string;
  status: string;
  score_units: number;
  max_score_units: number;
  answers: RecognizedAnswer[];
  revision: number;
  grading_version?: string;
  capture_id: string;
};
export type Results = {
  items: Result[];
  summary: {
    expected: number;
    received: number;
    needs_review: number;
    missing: number;
    average_units: number;
    max_score_units: number;
  };
};
export type DocumentItem = {
  id: string;
  filename: string;
  status: string;
  message?: string;
  error?: string;
  created_at?: string;
  content_type?: string;
  total_pages?: number;
  prepared_pages?: number;
  completed_pages?: number;
  current_page?: number;
  progress?: number;
  page_errors?: { page_number: number; message: string }[];
  artifact?: {
    error?: string;
    requires_teacher_confirmation?: boolean;
    extraction?: {
      text: string;
      pages: number;
      source: string;
    };
    analysis?: {
      questions: {
        number: string;
        type: string;
        options: string[];
        answer: string[];
        score_units: number | null;
        source_page: number | null;
        review_flags: string[];
      }[];
      warnings: string[];
      input_tokens: number;
      output_tokens: number;
      model: string;
    };
  };
};
export type GeometryBox = {
  x: number;
  y: number;
  width: number;
  height: number;
};
export type Manifest = {
  student_number_digits?: number;
  student_bubbles?: {
    column: number;
    digit: number;
    x: number;
    y: number;
    r: number;
  }[];
  bubbles?: {
    question_id: string;
    option: string;
    x: number;
    y: number;
    r: number;
  }[];
  questions?: { id: string; number: string; type: string; options: string[] }[];
  black_reference?: GeometryBox;
  white_reference?: GeometryBox;
  version: string;
  width_mm: number;
  height_mm: number;
  markers: {
    x: number;
    y: number;
  }[];
  coordinate_space?: "normalized";
  marker_size?: number;
  qr_box?: {
    x: number;
    y: number;
    width: number;
    height: number;
  };
};
