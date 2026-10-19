import { useCallback, useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { api } from "../api";
import { ErrorText, StageSelect } from "../components";
import { STAGES } from "../types";
import type { Customer, Stage } from "../types";

// A board with one column per stage. Move a card by changing its stage in the dropdown.
export default function PipelinePage() {
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      setCustomers(await api.listCustomers("", ""));
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load the pipeline");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function move(c: Customer, stage: string) {
    const previous = customers;
    // Update the screen right away, then tell the server. If it fails, put the card back.
    setCustomers((list) => list.map((x) => (x.id === c.id ? { ...x, stage: stage as Stage } : x)));
    try {
      await api.setStage(c.id, stage as Stage);
    } catch (err) {
      setCustomers(previous);
      setError(err instanceof Error ? err.message : "Could not move the customer");
    }
  }

  return (
    <>
      <div className="page-head">
        <h2>Pipeline</h2>
      </div>
      <ErrorText message={error} />
      {loading ? (
        <p className="muted">Loading...</p>
      ) : (
        <div className="board">
          {STAGES.map((stage) => {
            const items = customers.filter((c) => c.stage === stage);
            return (
              <section key={stage} className="column">
                <h3>
                  <span className={`badge badge-${stage}`}>{stage}</span> <span className="muted">{items.length}</span>
                </h3>
                {items.length === 0 && <p className="muted small">Empty</p>}
                {items.map((c) => (
                  <article key={c.id} className="card pipeline-card">
                    <Link to={`/customers/${c.id}`}>
                      <strong>{c.name}</strong>
                    </Link>
                    {c.company && <div className="muted small">{c.company}</div>}
                    <StageSelect value={c.stage} onChange={(s) => move(c, s)} />
                  </article>
                ))}
              </section>
            );
          })}
        </div>
      )}
    </>
  );
}
