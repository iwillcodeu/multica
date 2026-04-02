import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

// Mock next/navigation
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/login",
  useSearchParams: () => new URLSearchParams(),
}));

// Mock auth store
const mockSendCode = vi.fn();
const mockVerifyCode = vi.fn();
const mockLoginPassword = vi.fn();
vi.mock("@/features/auth", () => ({
  useAuthStore: (selector: (s: any) => any) =>
    selector({
      user: null,
      isLoading: false,
      sendCode: mockSendCode,
      verifyCode: mockVerifyCode,
      loginPassword: mockLoginPassword,
    }),
}));

// Mock workspace store
const mockHydrateWorkspace = vi.fn();
vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: (selector: (s: any) => any) =>
    selector({
      hydrateWorkspace: mockHydrateWorkspace,
    }),
}));

// Mock api
vi.mock("@/shared/api", () => ({
  api: {
    listWorkspaces: vi.fn().mockResolvedValue([]),
    verifyCode: vi.fn(),
    loginPassword: vi.fn(),
    setToken: vi.fn(),
    getMe: vi.fn(),
  },
}));

import LoginPage from "./page";

describe("LoginPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders password sign-in by default", () => {
    render(<LoginPage />);

    expect(screen.getByText("Multica")).toBeInTheDocument();
    expect(screen.getByText("AI-native task management")).toBeInTheDocument();
    expect(screen.getByLabelText("Email")).toBeInTheDocument();
    expect(screen.getByLabelText("Password")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Sign in" })).toBeInTheDocument();
  });

  it("does not call sendCode when email is empty on email-code tab", async () => {
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Email code" }));
    await user.click(screen.getByRole("button", { name: "Continue with email" }));
    expect(mockSendCode).not.toHaveBeenCalled();
  });

  it("calls sendCode with email on submit in email-code mode", async () => {
    mockSendCode.mockResolvedValueOnce(undefined);
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Email code" }));
    await user.type(screen.getByLabelText("Email"), "test@multica.ai");
    await user.click(screen.getByRole("button", { name: "Continue with email" }));

    await waitFor(() => {
      expect(mockSendCode).toHaveBeenCalledWith("test@multica.ai");
    });
  });

  it("shows 'Sending code...' while submitting", async () => {
    mockSendCode.mockReturnValueOnce(new Promise(() => {}));
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Email code" }));
    await user.type(screen.getByLabelText("Email"), "test@multica.ai");
    await user.click(screen.getByRole("button", { name: "Continue with email" }));

    await waitFor(() => {
      expect(screen.getByText("Sending code...")).toBeInTheDocument();
    });
  });

  it("shows verification code step after sending code", async () => {
    mockSendCode.mockResolvedValueOnce(undefined);
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Email code" }));
    await user.type(screen.getByLabelText("Email"), "test@multica.ai");
    await user.click(screen.getByRole("button", { name: "Continue with email" }));

    await waitFor(() => {
      expect(screen.getByText("Check your email")).toBeInTheDocument();
    });
  });

  it("shows error when sendCode fails", async () => {
    mockSendCode.mockRejectedValueOnce(new Error("Network error"));
    const user = userEvent.setup();
    render(<LoginPage />);

    await user.click(screen.getByRole("button", { name: "Email code" }));
    await user.type(screen.getByLabelText("Email"), "test@multica.ai");
    await user.click(screen.getByRole("button", { name: "Continue with email" }));

    await waitFor(() => {
      expect(screen.getByText("Network error")).toBeInTheDocument();
    });
  });
});
