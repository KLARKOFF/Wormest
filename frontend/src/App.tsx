import { Navigate, Route, Routes } from "react-router";
import { useAuth } from "./auth";
import { AuthPage } from "./pages/AuthPage";
import { Layout } from "./pages/Layout";
import { NoteEditor } from "./pages/NoteEditor";
import { EmptyState } from "./pages/EmptyState";

export function App() {
  const { user, loading } = useAuth();

  if (loading) return <div className="splash" aria-busy="true" />;

  if (!user) {
    return (
      <Routes>
        <Route path="/login" element={<AuthPage mode="login" />} />
        <Route path="/register" element={<AuthPage mode="register" />} />
        <Route path="*" element={<Navigate to="/login" replace />} />
      </Routes>
    );
  }

  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<EmptyState />} />
        <Route path="notes/:id" element={<NoteEditor />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
