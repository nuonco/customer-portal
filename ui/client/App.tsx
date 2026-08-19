import { useEffect, useState } from "react";

type HealthState = "idle" | "ok" | "error";

export function App() {
  const [health, setHealth] = useState<HealthState>("idle");

  useEffect(() => {
    fetch("/livez")
      .then((res) => {
        if (!res.ok) throw new Error("BFF not healthy");
        setHealth("ok");
      })
      .catch(() => setHealth("error"));
  }, []);

  return (
    <main className="app-shell">
      <h1>Customer Dashboard React Client</h1>
      <p>This is the new React frontend base. Existing Templ + HTMX pages stay untouched.</p>
      <p>
        BFF health: <strong>{health}</strong>
      </p>
    </main>
  );
}
