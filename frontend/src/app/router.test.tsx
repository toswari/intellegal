import { cleanup, render, screen } from "@testing-library/react";
import { Navigate, RouterProvider, createMemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";

afterEach(() => {
  cleanup();
});

vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return {
    ...actual,
    apiClient: {
      listContracts: vi.fn().mockResolvedValue({ items: [], limit: 200, offset: 0, total: 0 }),
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
      getDocument: vi.fn().mockResolvedValue({
        id: "00000000-0000-4000-8000-000000000000",
        filename: "contract.pdf",
        mime_type: "application/pdf",
        status: "indexed",
        created_at: "2026-01-01T00:00:00Z",
        updated_at: "2026-01-01T00:00:00Z"
      }),
      getDocumentText: vi.fn().mockResolvedValue({
        document_id: "00000000-0000-4000-8000-000000000000",
        filename: "contract.pdf",
        text: "test",
        has_text: true
      }),
      searchContractSections: vi.fn().mockResolvedValue({ items: [] }),
      createDocument: vi.fn(),
      deleteDocument: vi.fn(),
      startClausePresenceCheck: vi.fn(),
      startCompanyNameCheck: vi.fn()
    }
  };
});
import { AppShell } from "./AppShell";
import { AuditPage } from "../pages/AuditPage";
import { ChecksPage } from "../pages/ChecksPage";
import { ContractsPage } from "../pages/ContractsPage";
import { CompareContractsPage } from "../pages/CompareContractsPage";
import { DashboardPage } from "../pages/DashboardPage";
import { NewContractPage } from "../pages/NewContractPage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { ResultsPage } from "../pages/ResultsPage";
import { SearchPage } from "../pages/SearchPage";

function renderAt(path: string) {
  const router = createMemoryRouter(
    [
      {
        path: "/",
        element: <AppShell />,
        children: [
          { index: true, element: <DashboardPage /> },
          { path: "search", element: <SearchPage /> },
          { path: "contracts", element: <ContractsPage /> },
          { path: "contracts/new", element: <NewContractPage /> },
          { path: "contracts/compare", element: <CompareContractsPage /> },
          { path: "guidelines", element: <ChecksPage /> },
          { path: "checks", element: <Navigate to="/guidelines" replace /> },
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
    expect(screen.getByRole("link", { name: "Guidelines" })).toHaveAttribute("href", "/guidelines");
    expect(screen.queryByRole("link", { name: "Results" })).not.toBeInTheDocument();
  });

  it("renders guidelines route from memory router navigation", () => {
    renderAt("/guidelines");

    expect(screen.getByRole("heading", { level: 2, name: "Guidelines" })).toBeVisible();
    expect(screen.getByText("New Guideline Run")).toBeVisible();
    expect(screen.getByText("Past Guideline Runs")).toBeVisible();
  });

  it("redirects legacy checks route to guidelines", async () => {
    renderAt("/checks");

    expect(await screen.findByRole("heading", { level: 2, name: "Guidelines" })).toBeVisible();
  });

  it("redirects legacy results route to guidelines", async () => {
    renderAt("/results?checkId=00000000-0000-4000-8000-000000000000");

    expect(await screen.findByRole("heading", { level: 2, name: "Guidelines" })).toBeVisible();
    expect(screen.getByText("Past Guideline Runs")).toBeVisible();
  });

  it("renders not found route for unknown paths", () => {
    renderAt("/missing-page");

    expect(screen.getByRole("heading", { level: 2, name: "Page not found" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Go to dashboard" })).toHaveAttribute("href", "/");
  });
});
