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
export type Manifest = {
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
