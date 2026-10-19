import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../api";
import { CustomerForm, ErrorText, StageBadge, StageSelect, formatDate } from "../components";
import { STAGES } from "../types";
import type { Customer, Stats } from "../types";

export default function CustomersPage() {
  const navigate = useNavigate();
  const [customers, setCustomers] = useState<Customer[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [query, setQuery] = useState("");
  const [search, setSearch] = useState(""); // the query we actually sent (debounced)
  const [stage, setStage] = useState("");
  const [adding, setAdding] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Wait 300ms after the last keystroke before searching, so we don't call the API on every letter.
  useEffect(() => {
    const t = setTimeout(() => setSearch(query), 300);
    return () => clearTimeout(t);
  }, [query]);

  const load = useCallback(async () => {
    try {
      const [list, s] = await Promise.all([api.listCustomers(search, stage), api.stats()]);
      setCustomers(list);
      setStats(s);
      setError("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load customers");
    } finally {
      setLoading(false);
    }
  }, [search, stage]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <>
      <div className="page-head">
        <h2>Customers {stats && <span className="muted">({stats.total})</span>}</h2>
        <button onClick={() => setAdding((v) => !v)}>{adding ? "Close" : "+ Add customer"}</button>
      </div>

      {stats && (
        <div className="stat-row">
          {STAGES.map((s) => (
            <button
              key={s}
              className={`stat ${stage === s ? "active" : ""}`}
              onClick={() => setStage(stage === s ? "" : s)}
              title={`Show only ${s}`}
            >
              <span className="stat-num">{stats.by_stage[s]}</span>
              <span className="stat-label">{s}</span>
            </button>
          ))}
        </div>
      )}

      {adding && (
        <div className="card">
          <h3>New customer</h3>
          <CustomerForm
            submitLabel="Add customer"
            withStage
            onCancel={() => setAdding(false)}
            onSubmit={async (input) => {
              const created = await api.createCustomer(input);
              navigate(`/customers/${created.id}`);
            }}
          />
        </div>
      )}

      <div className="toolbar">
        <input
          type="search"
          placeholder="Search name, email or company"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <StageSelect value={stage} onChange={setStage} allowAll />
      </div>

      <ErrorText message={error} />

      <div className="card table-card">
        {loading ? (
          <p className="muted pad">Loading...</p>
        ) : customers.length === 0 ? (
          <p className="muted pad">
            {search || stage ? "No customers match your filters." : "No customers yet. Add your first one!"}
          </p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Company</th>
                <th>Contact</th>
                <th>Stage</th>
                <th>Added</th>
              </tr>
            </thead>
            <tbody>
              {customers.map((c) => (
                <tr key={c.id}>
                  <td>
                    <Link to={`/customers/${c.id}`}>{c.name}</Link>
                  </td>
                  <td>{c.company || "-"}</td>
                  <td>{c.email || c.phone || "-"}</td>
                  <td>
                    <StageBadge stage={c.stage} />
                  </td>
                  <td className="muted">{formatDate(c.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
