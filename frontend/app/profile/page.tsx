"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000";

type Profile = { id: number; email: string; isAdmin: boolean; created_at: string };

export default function ProfilePage() {
  const [profile, setProfile] = useState<Profile | null>(null);
  const [error, setError] = useState("");
  const [retention, setRetention] = useState<number | null>(null);

  useEffect(() => {
    fetch(`${API_BASE_URL}/auth/me`, { credentials: "include" })
      .then(async (response) => {
        if (!response.ok) throw new Error("Could not load profile.");
        const loaded = (await response.json()) as Profile;
        setProfile(loaded);
        if (loaded.isAdmin) {
          fetch(`${API_BASE_URL}/storage`, { credentials: "include" }).then((value) => value.ok ? value.json() : null).then((value: { orphanRetentionHours?: number } | null) => setRetention(value?.orphanRetentionHours ?? null)).catch(() => undefined);
        }
      })
      .catch((reason: unknown) => setError(reason instanceof Error ? reason.message : "Could not load profile."));
  }, []);

  return <main className="min-h-screen px-6 py-10"><section className="mx-auto max-w-xl rounded-lg border border-gray-200 bg-white p-6 shadow-sm">
    <div className="flex items-center justify-between"><h1 className="text-2xl font-semibold text-gray-950">Profile</h1><Link className="text-sm font-medium underline" href="/dashboard">Back to dashboard</Link></div>
    <p className="mt-2 text-sm text-gray-600">Read-only account information. DataGuardian does not expose secrets or authentication tokens here.</p>
    {error ? <p className="mt-5 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</p> : null}
    {profile ? <dl className="mt-6 divide-y divide-gray-200 rounded-md border border-gray-200">
      <div className="p-4"><dt className="text-xs font-semibold uppercase text-gray-500">Username</dt><dd className="mt-1 text-gray-900">{profile.email}</dd></div>
      <div className="p-4"><dt className="text-xs font-semibold uppercase text-gray-500">Role</dt><dd className="mt-1 text-gray-900">{profile.isAdmin ? "Local administrator" : "User"}</dd></div>
      <div className="p-4"><dt className="text-xs font-semibold uppercase text-gray-500">Created</dt><dd className="mt-1 text-gray-900">{new Date(profile.created_at).toLocaleString()}</dd></div>
      <div className="p-4"><dt className="text-xs font-semibold uppercase text-gray-500">Environment</dt><dd className="mt-1 text-gray-900">{process.env.NEXT_PUBLIC_ENVIRONMENT ?? "development"} · version 0.1.0</dd></div>
      {profile.isAdmin ? <div className="p-4"><dt className="text-xs font-semibold uppercase text-gray-500">Orphan retention</dt><dd className="mt-1 text-gray-900">{retention === null ? "Not available" : `${retention} hours`}</dd></div> : null}
      <div className="p-4"><dt className="text-xs font-semibold uppercase text-gray-500">Local preferences</dt><dd className="mt-1 text-gray-900">Theme preferences are stored only in this browser.</dd></div>
    </dl> : !error ? <p className="mt-5 text-sm text-gray-600">Loading profile...</p> : null}
  </section></main>;
}
