import { createBrowserRouter } from "react-router-dom";
import { AppShell } from "./AppShell";
import { AuditPage } from "../pages/AuditPage";
import { ChecksPage } from "../pages/ChecksPage";
import { ContractsPage } from "../pages/ContractsPage";
import { DashboardPage } from "../pages/DashboardPage";
import { NewContractPage } from "../pages/NewContractPage";
import { NotFoundPage } from "../pages/NotFoundPage";
import { ResultsPage } from "../pages/ResultsPage";

export const router = createBrowserRouter([
  {
    path: "/",
    element: <AppShell />,
    children: [
      { index: true, element: <DashboardPage /> },
      { path: "contracts", element: <ContractsPage /> },
      { path: "contracts/new", element: <NewContractPage /> },
      { path: "checks", element: <ChecksPage /> },
      { path: "results", element: <ResultsPage /> },
      { path: "audit", element: <AuditPage /> }
    ]
  },
  {
    path: "*",
    element: <NotFoundPage />
  }
]);
