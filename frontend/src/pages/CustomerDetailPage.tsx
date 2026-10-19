import { useCallback, useEffect, useState } from "react";
import type { FormEvent } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { api } from "../api";
import { CustomerForm, ErrorText, StageSelect, formatDate } from "../components";
import { KINDS } from "../types";
import type { Customer, Interaction, Kind, Stage } from "../types";

export default function CustomerDetailPage() {
  const { id } = useParams();
  const customerId = Number(id);
  const navigate = useNavigate();

  const [customer, setCustomer] = useState<Customer | null>(null);
  const [interactions, setInteractions] = useState<Interaction[]>([]);
  const [editing, setEditing] = useState(false);
  const [error, setError] = useState("");
  const [notFound, setNotFound] = useState(false);

  const [kind, setKind] = useState<Kind>("note");
  const [content, setContent] = useState("");
  const [logError, setLogError] = useState("");

  const load = useCallback(async () => {
    try {
      const [c, list] = await Promise.all([api.getCustomer(customerId), api.listInteractions(customerId)]);
      setCustomer(c);
      setInteractions(list);
    } catch (err) {
      if (err instanceof Error && "status" in err && (err as { status: number }).status === 404) {
        setNotFound(true);
      } else {
        setError(err instanceof Error ? err.message : "Could not load customer");
      }
    }
  }, [customerId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function changeStage(stage: string) {
    try {
      setCustomer(await api.setStage(customerId, stage as Stage));
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not change stage");
    }
  }

  async function removeCustomer() {
    if (!window.confirm(`Delete ${customer?.name}? Their interaction history will be deleted too.`)) return;
    try {
      await api.deleteCustomer(customerId);
      navigate("/");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not delete customer");
    }
  }

  async function addLog(e: FormEvent) {
    e.preventDefault();
    setLogError("");
    try {
      const created = await api.addInteraction(customerId, kind, content);
      setInteractions((prev) => [created, ...prev]);
      setContent("");
    } catch (err) {
      setLogError(err instanceof Error ? err.message : "Could not save");
    }
  }

  async function removeLog(logId: number) {
    try {
      await api.deleteInteraction(logId);
      setInteractions((prev) => prev.filter((i) => i.id !== logId));
    } catch (err) {
      setLogError(err instanceof Error ? err.message : "Could not delete");
    }
  }

  if (notFound) {
    return (
      <div className="card">
        <p>This customer does not exist (or it is not yours).</p>
        <Link to="/">Back to customers</Link>
      </div>
    );
  }
  if (!customer) return error ? <ErrorText message={error} /> : <p className="muted">Loading...</p>;

  return (
    <>
      <p>
        <Link to="/">&larr; All customers</Link>
      </p>

      <div className="page-head">
        <h2>{customer.name}</h2>
        <div className="row">
          <label className="inline">
            Stage <StageSelect value={customer.stage} onChange={changeStage} />
          </label>
          <button className="secondary" onClick={() => setEditing((v) => !v)}>
            {editing ? "Close" : "Edit"}
          </button>
          <button className="danger" onClick={removeCustomer}>
            Delete
          </button>
        </div>
      </div>
      <ErrorText message={error} />

      <div className="card">
        {editing ? (
          <CustomerForm
            initial={customer}
            submitLabel="Save changes"
            onCancel={() => setEditing(false)}
            onSubmit={async (input) => {
              setCustomer(await api.updateCustomer(customerId, input));
              setEditing(false);
            }}
          />
        ) : (
          <dl className="details">
            <dt>Company</dt>
            <dd>{customer.company || "-"}</dd>
            <dt>Email</dt>
            <dd>{customer.email ? <a href={`mailto:${customer.email}`}>{customer.email}</a> : "-"}</dd>
            <dt>Phone</dt>
            <dd>{customer.phone || "-"}</dd>
            <dt>Added</dt>
            <dd>{formatDate(customer.created_at)}</dd>
          </dl>
        )}
      </div>

      <h3>Interaction history</h3>
      <form className="card log-form" onSubmit={addLog}>
        <div className="row">
          <select value={kind} onChange={(e) => setKind(e.target.value as Kind)}>
            {KINDS.map((k) => (
              <option key={k} value={k}>
                {k}
              </option>
            ))}
          </select>
          <input
            className="grow"
            placeholder="What happened? e.g. Called to discuss pricing"
            value={content}
            onChange={(e) => setContent(e.target.value)}
            required
            maxLength={4000}
          />
          <button type="submit">Log</button>
        </div>
        <ErrorText message={logError} />
      </form>

      {interactions.length === 0 ? (
        <p className="muted">No interactions logged yet.</p>
      ) : (
        <ul className="timeline">
          {interactions.map((i) => (
            <li key={i.id} className="card">
              <div className="timeline-head">
                <span className={`kind kind-${i.kind}`}>{i.kind}</span>
                <span className="muted small">{formatDate(i.created_at)}</span>
                <button className="link danger-text" onClick={() => removeLog(i.id)}>
                  delete
                </button>
              </div>
              <p>{i.content}</p>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
