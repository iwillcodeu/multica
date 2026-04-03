"use client";

import { Suspense, useState, useEffect, useCallback } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { useAuthStore } from "@/features/auth";
import { useWorkspaceStore } from "@/features/workspace";
import {
  resolveDefaultBoardPath,
  shouldResolveFirstProject,
  useProjectStore,
} from "@/features/projects";
import { api } from "@/shared/api";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
  CardContent,
  CardFooter,
} from "@/components/ui/card";
import { Eye, EyeOff } from "lucide-react";
import { Input } from "@/components/ui/input";
import {
  InputGroup,
  InputGroupAddon,
  InputGroupButton,
  InputGroupInput,
} from "@/components/ui/input-group";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import {
  InputOTP,
  InputOTPGroup,
  InputOTPSlot,
} from "@/components/ui/input-otp";
import type { User } from "@/shared/types";

function validateCliCallback(cliCallback: string): boolean {
  try {
    const cbUrl = new URL(cliCallback);
    if (cbUrl.protocol !== "http:") return false;
    if (cbUrl.hostname !== "localhost" && cbUrl.hostname !== "127.0.0.1")
      return false;
    return true;
  } catch {
    return false;
  }
}

function redirectToCliCallback(
  cliCallback: string,
  token: string,
  cliState: string
) {
  const separator = cliCallback.includes("?") ? "&" : "?";
  window.location.href = `${cliCallback}${separator}token=${encodeURIComponent(token)}&state=${encodeURIComponent(cliState)}`;
}

function LoginPageContent() {
  const router = useRouter();
  const user = useAuthStore((s) => s.user);
  const isLoading = useAuthStore((s) => s.isLoading);
  const sendCode = useAuthStore((s) => s.sendCode);
  const verifyCode = useAuthStore((s) => s.verifyCode);
  const loginPassword = useAuthStore((s) => s.loginPassword);
  const hydrateWorkspace = useWorkspaceStore((s) => s.hydrateWorkspace);
  const searchParams = useSearchParams();

  // Already authenticated — redirect to dashboard (first project board when `next` is /projects)
  useEffect(() => {
    if (isLoading || !user || searchParams.get("cli_callback")) return;

    const next = searchParams.get("next");
    if (!shouldResolveFirstProject(next)) {
      router.replace(next!);
      return;
    }

    const go = () => {
      router.replace(resolveDefaultBoardPath(next));
    };

    if (!useProjectStore.getState().loading) {
      go();
      return;
    }

    const unsub = useProjectStore.subscribe((s) => {
      if (!s.loading) {
        go();
        unsub();
      }
    });
    return () => unsub();
  }, [isLoading, user, router, searchParams]);

  const [step, setStep] = useState<"email" | "code" | "cli_confirm">("email");
  const [authMode, setAuthMode] = useState<"password" | "code">("password");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [passwordVisible, setPasswordVisible] = useState(false);
  const [code, setCode] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [cooldown, setCooldown] = useState(0);
  const [existingUser, setExistingUser] = useState<User | null>(null);

  // Check for existing session when CLI callback is present.
  useEffect(() => {
    const cliCallback = searchParams.get("cli_callback");
    if (!cliCallback) return;

    const token = localStorage.getItem("multica_token");
    if (!token) return;

    if (!validateCliCallback(cliCallback)) return;

    // Verify the existing token is still valid.
    api.setToken(token);
    api
      .getMe()
      .then((user) => {
        setExistingUser(user);
        setStep("cli_confirm");
      })
      .catch(() => {
        // Token expired/invalid — clear and fall through to normal login.
        api.setToken(null);
        localStorage.removeItem("multica_token");
      });
  }, [searchParams]);

  useEffect(() => {
    if (cooldown <= 0) return;
    const timer = setTimeout(() => setCooldown((c) => c - 1), 1000);
    return () => clearTimeout(timer);
  }, [cooldown]);

  const handleCliAuthorize = async () => {
    const cliCallback = searchParams.get("cli_callback");
    const token = localStorage.getItem("multica_token");
    if (!cliCallback || !token) return;
    const cliState = searchParams.get("cli_state") || "";
    setSubmitting(true);
    redirectToCliCallback(cliCallback, token, cliState);
  };

  const handlePasswordLogin = async (e?: React.FormEvent) => {
    e?.preventDefault();
    if (!email.trim() || !password) {
      setError("Email and password are required");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      const cliCallback = searchParams.get("cli_callback");
      if (cliCallback) {
        if (!validateCliCallback(cliCallback)) {
          setError("Invalid callback URL");
          setSubmitting(false);
          return;
        }
        const { token } = await api.loginPassword(email.trim(), password);
        const cliState = searchParams.get("cli_state") || "";
        redirectToCliCallback(cliCallback, token, cliState);
        return;
      }

      await loginPassword(email.trim(), password);
      const wsList = await api.listWorkspaces();
      await hydrateWorkspace(wsList);
      router.push(resolveDefaultBoardPath(searchParams.get("next")));
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Sign-in failed",
      );
      setSubmitting(false);
    }
  };

  const handleSendCode = async (e?: React.FormEvent) => {
    e?.preventDefault();
    if (!email) {
      setError("Email is required");
      return;
    }
    setError("");
    setSubmitting(true);
    try {
      await sendCode(email);
      setStep("code");
      setCode("");
      setCooldown(10);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Failed to send code. Make sure the server is running."
      );
    } finally {
      setSubmitting(false);
    }
  };

  const handleVerifyCode = useCallback(
    async (value: string) => {
      if (value.length !== 6) return;
      setError("");
      setSubmitting(true);
      try {
        const cliCallback = searchParams.get("cli_callback");
        if (cliCallback) {
          if (!validateCliCallback(cliCallback)) {
            setError("Invalid callback URL");
            setSubmitting(false);
            return;
          }
          const { token } = await api.verifyCode(email, value);
          const cliState = searchParams.get("cli_state") || "";
          redirectToCliCallback(cliCallback, token, cliState);
          return;
        }

        await verifyCode(email, value);
        const wsList = await api.listWorkspaces();
        await hydrateWorkspace(wsList);
        router.push(resolveDefaultBoardPath(searchParams.get("next")));
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Invalid or expired code"
        );
        setCode("");
        setSubmitting(false);
      }
    },
    [email, verifyCode, hydrateWorkspace, router, searchParams]
  );

  const handleResend = async () => {
    if (cooldown > 0) return;
    setError("");
    try {
      await sendCode(email);
      setCooldown(10);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to resend code"
      );
    }
  };

  // CLI confirm step: user is already logged in, just authorize.
  if (step === "cli_confirm" && existingUser) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Authorize CLI</CardTitle>
            <CardDescription>
              Allow the CLI to access Multica as{" "}
              <span className="font-medium text-foreground">
                {existingUser.email}
              </span>
              ?
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-3">
            <Button
              onClick={handleCliAuthorize}
              disabled={submitting}
              className="w-full"
              size="lg"
            >
              {submitting ? "Authorizing..." : "Authorize"}
            </Button>
            <Button
              variant="ghost"
              className="w-full"
              onClick={() => {
                setExistingUser(null);
                setStep("email");
              }}
            >
              Use a different account
            </Button>
          </CardContent>
        </Card>
      </div>
    );
  }

  if (step === "code") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Card className="w-full max-w-sm">
          <CardHeader className="text-center">
            <CardTitle className="text-2xl">Check your email</CardTitle>
            <CardDescription>
              We sent a verification code to{" "}
              <span className="font-medium text-foreground">{email}</span>
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col items-center gap-4">
            <InputOTP
              maxLength={6}
              value={code}
              onChange={(value) => {
                setCode(value);
                if (value.length === 6) handleVerifyCode(value);
              }}
              disabled={submitting}
            >
              <InputOTPGroup>
                <InputOTPSlot index={0} />
                <InputOTPSlot index={1} />
                <InputOTPSlot index={2} />
                <InputOTPSlot index={3} />
                <InputOTPSlot index={4} />
                <InputOTPSlot index={5} />
              </InputOTPGroup>
            </InputOTP>
            {error && (
              <p className="text-sm text-destructive">{error}</p>
            )}
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <button
                type="button"
                onClick={handleResend}
                disabled={cooldown > 0}
                className="text-primary underline-offset-4 hover:underline disabled:text-muted-foreground disabled:no-underline disabled:cursor-not-allowed"
              >
                {cooldown > 0 ? `Resend in ${cooldown}s` : "Resend code"}
              </button>
            </div>
          </CardContent>
          <CardFooter>
            <Button
              variant="ghost"
              className="w-full"
              onClick={() => {
                setStep("email");
                setCode("");
                setError("");
              }}
            >
              Back
            </Button>
          </CardFooter>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex min-h-screen items-center justify-center">
      <Card className="w-full max-w-sm">
        <CardHeader className="text-center">
          <CardTitle className="text-2xl">Human and AI Working Together</CardTitle>
          <CardDescription>Turn coding agents into real teammates</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex rounded-lg border border-border p-0.5 text-sm">
            <button
              type="button"
              className={`flex-1 rounded-md py-2 font-medium transition-colors ${
                authMode === "password"
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:text-foreground"
              }`}
              onClick={() => {
                setAuthMode("password");
                setError("");
                setPasswordVisible(false);
              }}
            >
              Password for existing
            </button>
            <button
              type="button"
              className={`flex-1 rounded-md py-2 font-medium transition-colors ${
                authMode === "code"
                  ? "bg-muted text-foreground"
                  : "text-muted-foreground hover:text-foreground"
              }`}
              onClick={() => {
                setAuthMode("code");
                setError("");
              }}
            >
              Email code for new
            </button>
          </div>

          {authMode === "password" ? (
            <form id="login-password-form" onSubmit={handlePasswordLogin} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <Input
                  id="email"
                  type="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  autoComplete="email"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="password">Password</Label>
                <InputGroup>
                  <InputGroupInput
                    id="password"
                    type={passwordVisible ? "text" : "password"}
                    placeholder="Your account password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    required
                    autoComplete="current-password"
                  />
                  <InputGroupAddon align="inline-end">
                    <InputGroupButton
                      type="button"
                      variant="ghost"
                      size="icon-xs"
                      aria-label={passwordVisible ? "Hide password" : "Show password"}
                      onClick={() => setPasswordVisible((v) => !v)}
                    >
                      {passwordVisible ? (
                        <EyeOff className="h-4 w-4 text-muted-foreground" />
                      ) : (
                        <Eye className="h-4 w-4 text-muted-foreground" />
                      )}
                    </InputGroupButton>
                  </InputGroupAddon>
                </InputGroup>
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
            </form>
          ) : (
            <form id="login-form" onSubmit={handleSendCode} className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="email-code">Email</Label>
                <Input
                  id="email-code"
                  type="email"
                  placeholder="you@example.com"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                  autoComplete="email"
                />
              </div>
              <p className="text-xs text-muted-foreground">
                New users get a verification code and a personal workspace.
                新用户创建新的工作空间用邮件验证码初始化登录。
              </p>
              {error && <p className="text-sm text-destructive">{error}</p>}
            </form>
          )}
        </CardContent>
        <CardFooter>
          {authMode === "password" ? (
            <Button
              type="submit"
              form="login-password-form"
              disabled={submitting}
              className="w-full"
              size="lg"
            >
              {submitting ? "Signing in..." : "Sign in"}
            </Button>
          ) : (
            <Button
              type="submit"
              form="login-form"
              disabled={submitting}
              className="w-full"
              size="lg"
            >
              {submitting ? "Sending code..." : "Continue with email"}
            </Button>
          )}
        </CardFooter>
      </Card>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={null}>
      <LoginPageContent />
    </Suspense>
  );
}
