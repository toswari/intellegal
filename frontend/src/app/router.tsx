import { Navigate, createBrowserRouter } from "react-router-dom";
import { AppShell } from "./AppShell";
import { AuditPage } from "../pages/AuditPage";
import { ChecksPage } from "../pages/ChecksPage";
import { ContractsPage } from "../pages/ContractsPage";
import { CompareContractsPage } from "../pages/CompareContractsPage";
import { ContractEditPage } from "../pages/ContractEditPage";
import { ContractViewPage } from "../pages/ContractViewPage";
import { DashboardPage } from "../pages/DashboardPage";
import { NewContractPage } from "../pages/NewContractPage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { ResultsPage } from "../pages/ResultsPage";
import { SearchPage } from "../pages/SearchPage";

export const router = createBrowserRouter([
  {
    path: "/",
    element: <AppShell />,
    children: [
      { index: true, element: <DashboardPage /> },
      { path: "search", element: <SearchPage /> },
      { path: "contracts", element: <ContractsPage /> },
      { path: "contracts/new", element: <NewContractPage /> },
      { path: "contracts/files/:documentId", element: <ContractViewPage /> },
      { path: "contracts/:contractId/edit", element: <ContractEditPage /> },
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
]);
