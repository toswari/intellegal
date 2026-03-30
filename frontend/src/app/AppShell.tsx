import { NavLink, Outlet } from "react-router-dom";

type NavItem = {
  to: string;
  label: string;
  end?: boolean;
};

const navItems: NavItem[] = [
  { to: "/", label: "Dashboard", end: true },
  { to: "/contracts", label: "Contracts" },
  { to: "/checks", label: "Checks" },
  { to: "/results", label: "Results" },
  { to: "/audit", label: "Audit Log" }
];

export function AppShell() {
  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="brand">
          <div className="logo" aria-hidden="true">
            <span className="logo-dot" />
            <span className="logo-smile" />
          </div>
          <div>
            <p className="brand-kicker">Riverty Sandbox</p>
            <h1>Legal Document Intelligence</h1>
          </div>
        </div>
        <nav className="nav" aria-label="Primary">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) => (isActive ? "active" : undefined)}
            >
              {item.label}
            </NavLink>
          ))}
        </nav>
      </header>
      <main className="app-main">
        <Outlet />
      </main>
    </div>
  );
}
