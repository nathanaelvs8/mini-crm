import { useState } from "react";
import type { FormEvent } from "react";
import { STAGES } from "./types";
import type { Customer, CustomerInput, Stage } from "./types";

export function StageBadge({ stage }: { stage: Stage }) {
  return <span className={`badge badge-${stage}`}>{stage}</span>;
}

export function StageSelect({
  value,
  onChange,
  allowAll = false,
  disabled = false,
}: {
  value: string;
  onChange: (stage: string) => void;
  allowAll?: boolean;
  disabled?: boolean;
}) {
  return (
    <select value={value} onChange={(e) => onChange(e.target.value)} disabled={disabled}>
      {allowAll && <option value="">All stages</option>}
      {STAGES.map((s) => (
        <option key={s} value={s}>
          {s}
        </option>
      ))}
    </select>
  );
}

export function ErrorText({ message }: { message: string }) {
  return message ? <p className="error" role="alert">{message}</p> : null;
}

export function formatDate(iso: string): string {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

// One form used for both "add customer" and "edit customer".
export function CustomerForm({
  initial,
  submitLabel,
  withStage = false,
  onSubmit,
  onCancel,
}: {
  initial?: Customer;
  submitLabel: string;
  withStage?: boolean;
  onSubmit: (input: CustomerInput) => Promise<void>;
  onCancel?: () => void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [email, setEmail] = useState(initial?.email ?? "");
  const [phone, setPhone] = useState(initial?.phone ?? "");
  const [company, setCompany] = useState(initial?.company ?? "");
  const [stage, setStage] = useState<Stage>("lead");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError("");
    setBusy(true);
    try {
      await onSubmit({ name, email, phone, company, ...(withStage ? { stage } : {}) });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong");
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="form-grid" onSubmit={handleSubmit}>
      <label>
        Name *
        <input value={name} onChange={(e) => setName(e.target.value)} required maxLength={200} />
      </label>
      <label>
        Company
        <input value={company} onChange={(e) => setCompany(e.target.value)} maxLength={200} />
      </label>
      <label>
        Email
        <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} maxLength={200} />
      </label>
      <label>
        Phone
        <input value={phone} onChange={(e) => setPhone(e.target.value)} maxLength={50} />
      </label>
      {withStage && (
        <label>
          Stage
          <StageSelect value={stage} onChange={(s) => setStage(s as Stage)} />
        </label>
      )}
      <div className="form-actions">
        <button type="submit" disabled={busy}>
          {busy ? "Saving..." : submitLabel}
        </button>
        {onCancel && (
          <button type="button" className="secondary" onClick={onCancel}>
            Cancel
          </button>
        )}
      </div>
      <ErrorText message={error} />
    </form>
  );
}
