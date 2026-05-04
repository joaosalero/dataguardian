"use client";

import { FormEvent, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL ??
  process.env.NEXT_PUBLIC_API_BASE_URL ??
  "http://localhost:8000";
const IS_PRODUCTION = process.env.NEXT_PUBLIC_ENVIRONMENT === "prod";

async function readRegisterError(response: Response) {
  const responseText = await response.text();
  if (!responseText) {
    return "Could not create the account. Please try again.";
  }

  try {
    const parsed = JSON.parse(responseText) as { detail?: unknown };
    if (typeof parsed.detail === "string") {
      return parsed.detail;
    }
  } catch {
    return responseText;
  }

  return "Could not create the account. Please try again.";
}

export default function RegisterPage() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setMessage("");
    setLoading(true);

    try {
      const response = await fetch(`${API_BASE_URL}/auth/register`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ username, password }),
      });

      if (!response.ok) {
        if (response.status === 429) {
          throw new Error("Too many registration attempts. Please wait and try again.");
        }
        throw new Error(await readRegisterError(response));
      }

      setMessage("Account created. You can sign in now.");
      setUsername("");
      setPassword("");
      setTimeout(() => router.push("/login"), 800);
    } catch (err) {
      if (err instanceof TypeError) {
        setError("Backend is offline or unreachable. Start the API and try again.");
      } else {
        setError(err instanceof Error ? err.message : "Network error. Please try again.");
      }
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="flex min-h-screen items-center justify-center px-6 py-10">
      <section className="w-full max-w-sm rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
        <div className="mb-6">
          <h1 className="text-2xl font-semibold text-gray-950">Create account</h1>
          <p className="mt-1 text-sm text-gray-600">
            Register to create projects and run audits.
          </p>
          {!IS_PRODUCTION ? (
            <p className="mt-3 rounded-md bg-blue-50 px-3 py-2 text-xs text-blue-700">
              Dev login is also available: admin / admin123
            </p>
          ) : null}
        </div>

        <form className="space-y-4" onSubmit={handleSubmit}>
          <label className="block">
            <span className="text-sm font-medium text-gray-700">Username</span>
            <input
              className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
              disabled={loading}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              autoComplete="username"
              required
            />
          </label>

          <label className="block">
            <span className="text-sm font-medium text-gray-700">Password</span>
            <input
              className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
              disabled={loading}
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              autoComplete="new-password"
              required
            />
          </label>

          {error ? (
            <p className="rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">
              {error}
            </p>
          ) : null}

          {message ? (
            <p className="rounded-md bg-green-50 px-3 py-2 text-sm text-green-700">
              {message}
            </p>
          ) : null}

          <button
            className="w-full rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
            disabled={loading || !username.trim() || !password}
            type="submit"
          >
            {loading ? "Creating..." : "Create account"}
          </button>
        </form>
        <p className="mt-4 text-center text-sm text-gray-600">
          Already have an account?{" "}
          <Link className="font-medium text-gray-950 hover:underline" href="/login">
            Sign in
          </Link>
        </p>
      </section>
    </main>
  );
}
