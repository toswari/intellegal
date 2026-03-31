import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
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
      listContracts: vi.fn().mockResolvedValue({
        items: [
          {
            id: "contract-1",
            name: "Alpha",
            file_count: 1,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z"
          },
          {
            id: "contract-2",
            name: "Beta",
            file_count: 1,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z"
          }
        ],
        limit: 200,
        offset: 0,
        total: 2
      }),
      listDocuments: vi.fn().mockResolvedValue({
        items: [
          {
            id: "doc-1",
            contract_id: "contract-1",
            filename: "alpha.pdf",
            mime_type: "application/pdf",
            status: "indexed",
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z"
          },
          {
            id: "doc-2",
            contract_id: "contract-2",
            filename: "beta.pdf",
            mime_type: "application/pdf",
            status: "indexed",
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z"
          }
        ],
        limit: 200,
        offset: 0,
        total: 2
      }),
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
import { ContractsPage } from "../pages/ContractsPage";
import { CompareContractsPage } from "../pages/CompareContractsPage";
import { DashboardPage } from "../pages/DashboardPage";
import { GuidelineCreatePage } from "../pages/GuidelineCreatePage";
import { GuidelineRunPage } from "../pages/GuidelineRunPage";
import { GuidelinesPage } from "../pages/GuidelinesPage";
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
          { path: "guidelines", element: <GuidelinesPage /> },
          { path: "guidelines/new", element: <GuidelineCreatePage /> },
          { path: "guidelines/run", element: <GuidelineRunPage /> },
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
    expect(screen.getByText("Rules")).toBeVisible();
    expect(screen.getByText("Executions")).toBeVisible();
    expect(screen.getByRole("link", { name: "New Rule" })).toHaveAttribute("href", "/guidelines/new");
  });

  it("renders dedicated guideline creation route", () => {
    renderAt("/guidelines/new");

    expect(screen.getByRole("heading", { level: 2, name: "New Guideline Rule" })).toBeVisible();
    expect(screen.getByText("Rule Details")).toBeVisible();
    expect(screen.getByLabelText("Rule Name")).toBeVisible();
    expect(screen.getByLabelText("Rule Instructions")).toBeVisible();
    expect(screen.getByRole("link", { name: "Back to Guidelines" })).toHaveAttribute("href", "/guidelines");
  });

  it("renders dedicated guideline execution route", () => {
    renderAt("/guidelines/run");

    expect(screen.getByRole("heading", { level: 2, name: "Run Guideline" })).toBeVisible();
  });

  it("opens guideline creation from selected contracts", async () => {
    renderAt("/contracts");

    const alphaCheckbox = await screen.findByRole("checkbox", { name: "Select Alpha" });
    fireEvent.click(alphaCheckbox);

    fireEvent.click(await screen.findByRole("button", { name: "Run Guideline" }));

    expect(await screen.findByRole("heading", { level: 2, name: "Run Guideline" })).toBeVisible();
    await waitFor(() => {
      expect(screen.getByLabelText("Scope")).toHaveValue("selected");
    });
  });

  it("redirects legacy checks route to guidelines", async () => {
    renderAt("/checks");

    expect(await screen.findByRole("heading", { level: 2, name: "Guidelines" })).toBeVisible();
  });

  it("redirects legacy results route to guidelines", async () => {
    renderAt("/results?checkId=00000000-0000-4000-8000-000000000000");

    expect(await screen.findByRole("heading", { level: 2, name: "Guidelines" })).toBeVisible();
    expect(screen.getByText("Executions")).toBeVisible();
  });

  it("renders not found route for unknown paths", () => {
    renderAt("/missing-page");

    expect(screen.getByRole("heading", { level: 2, name: "Page not found" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Go to dashboard" })).toHaveAttribute("href", "/");
  });
});
