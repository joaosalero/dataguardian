"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000";

export default function DashboardPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");

  useEffect(() => {
    fetch(`${API_BASE_URL}/auth/me`, {
      credentials: "include",
    })
      .then((response) => {
        if (response.status === 401) {
          router.push("/login");
          return null;
        }
        if (!response.ok) {
          throw new Error("Could not verify the current session.");
        }
        return response.json() as Promise<{ email: string }>;
      })
      .then((profile) => {
        if (profile) {
          setEmail(profile.email);
        }
      })
      .catch(() => router.push("/login"));
  }, [router]);

  async function logout() {
    await fetch(`${API_BASE_URL}/auth/logout`, {
      method: "POST",
      credentials: "include",
    }).catch(() => undefined);
    router.push("/login");
  }

  return (
    <main className="min-h-screen px-6 py-8">
      <div className="mx-auto max-w-5xl">
        <header className="mb-8 flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
          <div>
            <h1 className="text-3xl font-semibold text-gray-950">Dashboard</h1>
            <p className="mt-1 text-sm text-gray-600">
              {email ? `Signed in as ${email}.` : "Verifying your session."}
            </p>
          </div>
          <button
            className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-800 hover:bg-gray-50"
            onClick={logout}
            type="button"
          >
            Sign out
          </button>
        </header>

        <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
          <h2 className="text-base font-semibold text-gray-950">Go backend active</h2>
          <p className="mt-2 text-sm text-gray-600">
            Authentication is handled by the consolidated Go API.
          </p>
        </section>
      </div>
    </main>
  );
}
