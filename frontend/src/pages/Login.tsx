import * as React from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError } from "@/lib/api";
import { HudPanel } from "@/components/HudPanel";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

export function Login() {
  const navigate = useNavigate();
  const [username, setUsername] = React.useState("nestor");
  const [password, setPassword] = React.useState("");
  const [code, setCode] = React.useState("");
  const [needTotp, setNeedTotp] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setLoading(true);
    try {
      if (!needTotp) {
        const res = await api.login(username, password);
        if (res.need_totp) {
          setNeedTotp(true);
          setLoading(false);
          return;
        }
        // dev bypass: token already set as cookie, navigate.
        navigate("/", { replace: true });
        return;
      }
      // TOTP step
      await api.totp(username, password, code);
      navigate("/", { replace: true });
    } catch (err) {
      if (err instanceof ApiError) {
        setError(
          err.status === 401
            ? "credenciales inválidas"
            : err.message || `error ${err.status}`
        );
      } else {
        setError("error de red");
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen w-full grid place-items-center relative">
      <div className="hud-grid" />
      <div className="hud-scan" />

      <div className="w-[420px] max-w-[92vw] relative z-10">
        {/* brand banner */}
        <div className="text-center mb-6">
          <div className="font-display font-bold text-[28px] tracking-hud text-[var(--color-fg)] leading-none">
            AGENT
            <span className="text-[var(--color-magenta)]">//</span>
            HUB
          </div>
          <div className="font-mono text-[10px] text-[var(--color-dim)] tracking-hud mt-2">
            HUD v0.1 · NODE-42 · UBUNTU 24.04
          </div>
        </div>

        <HudPanel title="acceso" sub="totp · sesión segura" accent="cyan">
          <form onSubmit={submit} className="flex flex-col gap-4 py-2">
            <div>
              <label className="block font-mono text-[10px] text-[var(--color-dim)] tracking-hud mb-1.5">
                USUARIO
              </label>
              <Input
                autoFocus={!needTotp}
                value={username}
                disabled={needTotp || loading}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="nestor"
                autoComplete="username"
              />
            </div>

            <div>
              <label className="block font-mono text-[10px] text-[var(--color-dim)] tracking-hud mb-1.5">
                CONTRASEÑA
              </label>
              <Input
                type="password"
                value={password}
                disabled={needTotp || loading}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="••••••••"
                autoComplete="current-password"
              />
            </div>

            {needTotp && (
              <div>
                <label className="block font-mono text-[10px] text-[var(--color-dim)] tracking-hud mb-1.5">
                  CÓDIGO TOTP
                </label>
                <Input
                  autoFocus
                  value={code}
                  disabled={loading}
                  onChange={(e) =>
                    setCode(e.target.value.replace(/\D/g, "").slice(0, 6))
                  }
                  placeholder="000000"
                  inputMode="numeric"
                  maxLength={6}
                />
              </div>
            )}

            {error && (
              <div
                className="px-3 py-2 clip-hud-sm font-mono text-[11px]"
                style={{
                  background: "rgba(255, 92, 122, 0.08)",
                  border: "1px solid rgba(255, 92, 122, 0.45)",
                  color: "var(--color-danger)",
                }}
              >
                ✗ {error}
              </div>
            )}

            <Button
              type="submit"
              variant="primary"
              size="lg"
              disabled={loading || !username || !password || (needTotp && code.length < 6)}
            >
              {loading
                ? "verificando…"
                : needTotp
                  ? "Validar TOTP"
                  : "Ingresar"}
            </Button>

            <div className="flex items-center justify-between font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight pt-1 border-t border-[var(--color-line)]">
              <span>● DEV_BYPASS_TOTP activo</span>
              <span>localhost:8093</span>
            </div>
          </form>
        </HudPanel>
      </div>
    </div>
  );
}
