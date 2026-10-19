export const STAGES = ["lead", "contacted", "negotiation", "won", "lost"] as const;
export type Stage = (typeof STAGES)[number];

export const KINDS = ["call", "email", "meeting", "note"] as const;
export type Kind = (typeof KINDS)[number];

export interface User {
  id: number;
  name: string;
  email: string;
}

export interface Customer {
  id: number;
  name: string;
  email: string;
  phone: string;
  company: string;
  stage: Stage;
  created_at: string;
  updated_at: string;
}

export interface CustomerInput {
  name: string;
  email: string;
  phone: string;
  company: string;
  stage?: Stage;
}

export interface Interaction {
  id: number;
  customer_id: number;
  kind: Kind;
  content: string;
  created_at: string;
}

export interface Stats {
  total: number;
  by_stage: Record<Stage, number>;
}

export interface AuthResponse {
  token: string;
  user: User;
}
