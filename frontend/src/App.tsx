import { BrowserRouter, Navigate, NavLink, Route, Routes } from "react-router-dom";
import { AuthProvider, useAuth } from "./auth";
import LoginPage from "./pages/LoginPage";
import CustomersPage from "./pages/CustomersPage";
import CustomerDetailPage from "./pages/CustomerDetailPage";
import PipelinePage from "./pages/PipelinePage";

function Shell() {
  const { user, logout } = useAuth();
  if (!user) return <LoginPage />;

  return (
    <>
      <header className="topbar">
        <strong className="brand">Mini CRM</strong>
        <nav>
          <NavLink to="/" end>
            Customers
          </NavLink>
          <NavLink to="/pipeline">Pipeline</NavLink>
        </nav>
        <div className="spacer" />
        <span className="muted small">{user.name}</span>
        <button className="secondary" onClick={logout}>
          Log out
        </button>
      </header>
      <main className="container">
        <Routes>
          <Route path="/" element={<CustomersPage />} />
          <Route path="/customers/:id" element={<CustomerDetailPage />} />
          <Route path="/pipeline" element={<PipelinePage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Shell />
      </BrowserRouter>
    </AuthProvider>
  );
}
