import { type FormEvent, useEffect, useState } from "react";
import { NavLink, Outlet, useLocation, useNavigate } from "react-router-dom";

type NavItem = {
  to: string;
  label: string;
  end?: boolean;
};

const navItems: NavItem[] = [
  { to: "/", label: "Dashboard", end: true },
  { to: "/contracts", label: "Contracts" },
  { to: "/guidelines", label: "Guidelines" },
  { to: "/audit", label: "Audit Log" }
];

export function AppShell() {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchQuery, setSearchQuery] = useState("");

  useEffect(() => {
    const query = new URLSearchParams(location.search).get("q") ?? "";
    setSearchQuery(query);
  }, [location.search]);

  const submitSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const query = searchQuery.trim();
    if (query.length < 2) {
      return;
    }
    const params = new URLSearchParams();
    params.set("q", query);
    navigate(`/search?${params.toString()}`);
  };

  const isContractDetailRoute = /^\/contracts\/[^/]+\/edit$/.test(location.pathname);

  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="app-header-top">
          <div className="brand">
            <img className="logo" src="/logo.webp" alt="IntelLegal logo" />
            <div className="brand-content">
              <p className="brand-kicker">IntelLegal</p>
              <h1>Legal Document Intelligence</h1>
            </div>
          </div>
          <div className="header-right">
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
            <div className="nav-search">
              <form className="global-search" onSubmit={submitSearch}>
                <div className="global-search-field">
                  <input
                    value={searchQuery}
                    onChange={(event) => setSearchQuery(event.target.value)}
                    placeholder="Search contracts..."
                    aria-label="Contract search"
                  />
                  <button type="submit" className="icon-button inline-icon" aria-label="Run search" title="Search">
                    <svg viewBox="0 0 24 24" width="16" height="16" aria-hidden="true" focusable="false">
                      <circle cx="11" cy="11" r="7" fill="none" stroke="currentColor" strokeWidth="2" />
                      <line x1="16.65" y1="16.65" x2="21" y2="21" stroke="currentColor" strokeWidth="2" />
                    </svg>
                  </button>
                </div>
              </form>
            </div>
          </div>
        </div>
      </header>
      <main className={`app-main${isContractDetailRoute ? " app-main-wide" : ""}`}>
        <Outlet />
      </main>
    </div>
  );
}
