import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { Login } from "@/pages/Login";
import { ChatMain } from "@/pages/ChatMain";
import { Projects } from "@/pages/Projects";
import { Agents } from "@/pages/Agents";
import { Topics } from "@/pages/Topics";
import { Subagents } from "@/pages/Subagents";
import { System } from "@/pages/System";
import { Vault } from "@/pages/Vault";
import { Diagrams } from "@/pages/Diagrams";
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
          <Route path="agents" element={<Agents />} />
          <Route path="agents/:id" element={<Agents />} />
          <Route path="topics" element={<Topics />} />
          <Route path="subagents" element={<Subagents />} />
          <Route path="system" element={<System />} />
          <Route path="vault" element={<Vault />} />
        </Route>

        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </BrowserRouter>
  );
}

export default App;
