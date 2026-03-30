import { render, screen } from "@testing-library/react";
import { RouterProvider, createMemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";

vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return {
    ...actual,
    apiClient: {
      listDocuments: vi.fn().mockResolvedValue({ items: [], limit: 200, offset: 0, total: 0 }),
      getCheckRun: vi.fn().mockResolvedValue({
        check_id: "00000000-0000-4000-8000-000000000000",
        status: "completed",
        check_type: "clause_presence",
        requested_at: "2026-01-01T00:00:00Z"
      }),
      getCheckResults: vi.fn().mockResolvedValue({
        check_id: "00000000-0000-4000-8000-000000000000",
        status: "completed",
        items: []
      }),
      createDocument: vi.fn(),
      startClausePresenceCheck: vi.fn(),
      startCompanyNameCheck: vi.fn()
    }
  };
});
import { AppShell } from "./AppShell";
import { AuditPage } from "../pages/AuditPage";
import { ChecksPage } from "../pages/ChecksPage";
import { ContractsPage } from "../pages/ContractsPage";
import { DashboardPage } from "../pages/DashboardPage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { ResultsPage } from "../pages/ResultsPage";

function renderAt(path: string) {
  const router = createMemoryRouter(
    [
      {
        path: "/",
        element: <AppShell />,
        children: [
          { index: true, element: <DashboardPage /> },
          { path: "contracts", element: <ContractsPage /> },
          { path: "checks", element: <ChecksPage /> },
          { path: "results", element: <ResultsPage /> },
          { path: "audit", element: <AuditPage /> }
        ]
      },
      {
        path: "*",
        element: <NotFoundPage />
      }
    ],
    { initialEntries: [path] }
  );

  render(<RouterProvider router={router} />);
}

describe("router", () => {
  it("renders dashboard content and primary navigation", () => {
    renderAt("/");

    expect(screen.getByRole("heading", { level: 1, name: "Legal Document Intelligence" })).toBeVisible();
    expect(screen.getByRole("heading", { level: 2, name: "Dashboard" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Contracts" })).toHaveAttribute("href", "/contracts");
    expect(screen.getByRole("link", { name: "Checks" })).toHaveAttribute("href", "/checks");
  });

  it("renders checks route from memory router navigation", () => {
    renderAt("/checks");

    expect(screen.getByRole("heading", { level: 2, name: "Checks" })).toBeVisible();
    expect(screen.getByText("New Check Wizard")).toBeVisible();
  });

  it("renders not found route for unknown paths", () => {
    renderAt("/missing-page");

    expect(screen.getByRole("heading", { level: 2, name: "Page not found" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Go to dashboard" })).toHaveAttribute("href", "/");
  });
});
