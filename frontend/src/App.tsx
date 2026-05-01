import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { Login } from "@/pages/Login";
import { ChatMain } from "@/pages/ChatMain";
import { Projects } from "@/pages/Projects";
import { System } from "@/pages/System";
import { Vault } from "@/pages/Vault";
import { Diagrams } from "@/pages/Diagrams";
import { Skills } from "@/pages/Skills";
import { Releases } from "@/pages/Releases";
import { Usage } from "@/pages/Usage";
import { AppShell } from "@/components/AppShell";

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/login" element={<Login />} />

        <Route element={<AppShell />}>
          <Route index element={<ChatMain />} />
          <Route path="projects" element={<Projects />} />
          <Route path="projects/:id" element={<Projects />} />
          <Route path="projects/:id/sessions/:sid" element={<Projects />} />
          <Route path="diagrams" element={<Diagrams />} />
          <Route path="system" element={<System />} />
          <Route path="usage" element={<Usage />} />
          <Route path="vault" element={<Vault />} />
          <Route path="skills" element={<Skills />} />
          <Route path="releases" element={<Releases />} />
          <Route path="agents/*" element={<Navigate to="/system" replace />} />
          <Route path="topics/*" element={<Navigate to="/system" replace />} />
          <Route path="subagents/*" element={<Navigate to="/system" replace />} />
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
