import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { RouterProvider, createMemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NewContractPage } from "./NewContractPage";

const apiMocks = vi.hoisted(() => ({
  createContract: vi.fn(),
  addContractFile: vi.fn()
}));

vi.mock("../api/client", async (importOriginal) => {
  const actual = await importOriginal<typeof import("../api/client")>();
  return {
    ...actual,
    apiClient: {
      ...actual.apiClient,
      createContract: apiMocks.createContract,
      addContractFile: apiMocks.addContractFile
    }
  };
});

function renderPage() {
  const router = createMemoryRouter(
    [
      { path: "/", element: <NewContractPage /> },
      { path: "/contracts/:contractId/edit", element: <div>Contract edit page</div> }
    ],
    { initialEntries: ["/"] }
  );

  render(<RouterProvider router={router} />);
}

describe("NewContractPage", () => {
  beforeEach(() => {
    apiMocks.createContract.mockResolvedValue({
      id: "contract-1",
      name: "vendor-agreement",
      file_count: 0,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z"
    });
    apiMocks.addContractFile.mockResolvedValue({
      id: "doc-1",
      contract_id: "contract-1",
      filename: "vendor-agreement.pdf",
      mime_type: "application/pdf",
      status: "indexed",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z"
    });
  });

  afterEach(() => {
    cleanup();
    apiMocks.createContract.mockReset();
    apiMocks.addContractFile.mockReset();
    window.localStorage.clear();
  });

  it("uses the first file name when contract name is left blank", async () => {
    renderPage();

    const file = new File(["pdf-content"], "vendor-agreement.pdf", { type: "application/pdf" });
    fireEvent.change(screen.getByLabelText("Contract Name (optional)"), { target: { value: "" } });
    fireEvent.change(screen.getByLabelText("Tags"), { target: { value: "MSA, Finance" } });
    const fileInput = document.querySelector('input[type="file"]');
    if (!(fileInput instanceof HTMLInputElement)) {
      throw new Error("Expected file input to be present.");
    }
    fireEvent.change(fileInput, { target: { files: [file] } });

    fireEvent.click(screen.getByRole("button", { name: "Create Contract" }));

    await waitFor(() => {
      expect(apiMocks.createContract).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "vendor-agreement",
          source_type: "upload",
          tags: ["MSA", "Finance"]
        }),
        expect.any(Object)
      );
    });
    expect(apiMocks.addContractFile).toHaveBeenCalledWith(
      "contract-1",
      expect.objectContaining({
        filename: "vendor-agreement.pdf",
        mime_type: "application/pdf",
        source_type: "upload",
        tags: ["MSA", "Finance"]
      }),
      expect.any(Object)
    );
  });
});
