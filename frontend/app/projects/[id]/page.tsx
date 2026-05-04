"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8000";

type Project = {
  id: number;
  name: string;
  description: string | null;
  created_at: string;
};

type AuditRun = {
  audit_id: number;
  project_id: number;
  status: string;
  score: number;
};

type AuditResult = {
  audit_id: number;
  score: number;
  findings: Array<{
    title: string;
    description: string;
    severity: string;
    recommendation: string;
  }>;
};

export default function ProjectPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const [project, setProject] = useState<Project | null>(null);
  const [history, setHistory] = useState<AuditRun[]>([]);
  const [lastAudit, setLastAudit] = useState<AuditResult | null>(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(true);
  const [running, setRunning] = useState(false);

  async function apiGet<T>(path: string): Promise<T> {
    const response = await fetch(`${API_BASE_URL}${path}`, {
      credentials: "include",
    });

    if (response.status === 401) {
      router.push("/login");
      throw new Error("Session expired");
    }

    if (!response.ok) {
      if (response.status === 404) {
        throw new Error("Project not found");
      }
      throw new Error("Something went wrong while loading project data.");
    }

    return response.json();
  }

  async function loadProject() {
    setError("");
    setMessage("");
    const [projectData, historyData] = await Promise.all([
      apiGet<Project>(`/projects/${params.id}`),
      apiGet<AuditRun[]>(`/projects/${params.id}/audit/history`),
    ]);
    setProject(projectData);
    setHistory(historyData);
  }

  useEffect(() => {
    loadProject()
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Something went wrong."),
      )
      .finally(() => setLoading(false));
  }, [params.id]);

  async function runAudit() {
    setRunning(true);
    setError("");
    setMessage("");
    try {
      const response = await fetch(
        `${API_BASE_URL}/projects/${params.id}/audit/run`,
        {
          method: "POST",
          credentials: "include",
        },
      );

      if (response.status === 401) {
        router.push("/login");
        return;
      }

      if (!response.ok) {
        setError("Could not run audit. Please try again.");
        return;
      }

      setLastAudit(await response.json());
      await loadProject();
      setMessage("Audit completed.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong.");
    } finally {
      setRunning(false);
    }
  }

  return (
    <main className="min-h-screen px-6 py-8">
      <div className="mx-auto max-w-5xl">
        <Link
          className="text-sm font-medium text-gray-600 hover:text-gray-950"
          href="/dashboard"
        >
          Back to projects
        </Link>

        {loading ? (
          <p className="mt-6 text-sm text-gray-600">Loading project...</p>
        ) : project ? (
          <>
            <header className="my-8 flex flex-col justify-between gap-4 sm:flex-row sm:items-start">
              <div>
                <h1 className="text-3xl font-semibold text-gray-950">
                  {project.name}
                </h1>
                <p className="mt-1 text-sm text-gray-600">
                  {project.description ?? "No description"}
                </p>
              </div>
              <button
                className="rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
                disabled={running}
                onClick={runAudit}
                type="button"
              >
                {running ? "Running..." : "Run audit"}
              </button>
            </header>

            {message ? (
              <p className="mb-4 rounded-md bg-green-50 px-3 py-2 text-sm text-green-700">
                {message}
              </p>
            ) : null}

            {error ? (
              <p className="mb-4 rounded-md bg-red-50 px-3 py-2 text-sm text-red-700">
                {error}
              </p>
            ) : null}

            {lastAudit ? (
              <section className="mb-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
                <div className="mb-4 flex items-center justify-between">
                  <h2 className="text-base font-semibold text-gray-950">
                    Latest audit
                  </h2>
                  <span className="rounded-md bg-gray-100 px-2 py-1 text-sm font-medium text-gray-800">
                    Score {lastAudit.score}
                  </span>
                </div>
                <div className="space-y-3">
                  {lastAudit.findings.map((finding, index) => (
                    <div
                      className="rounded-md border border-gray-200 p-3"
                      key={index}
                    >
                      <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                        <h3 className="text-sm font-medium text-gray-950">
                          {finding.title}
                        </h3>
                        <span className="text-xs font-semibold uppercase text-gray-500">
                          {finding.severity}
                        </span>
                      </div>
                      <p className="mt-1 text-sm text-gray-600">
                        {finding.description}
                      </p>
                    </div>
                  ))}
                </div>
              </section>
            ) : null}

            <section className="rounded-lg border border-gray-200 bg-white shadow-sm">
              <div className="border-b border-gray-200 p-5">
                <h2 className="text-base font-semibold text-gray-950">
                  Audit history
                </h2>
              </div>
              {history.length === 0 ? (
                <p className="p-5 text-sm text-gray-600">No audits yet.</p>
              ) : (
                <div className="divide-y divide-gray-200">
                  {history.map((audit) => (
                    <div
                      className="flex items-center justify-between p-5"
                      key={audit.audit_id}
                    >
                      <div>
                        <p className="text-sm font-medium text-gray-950">
                          Audit #{audit.audit_id}
                        </p>
                        <p className="text-sm text-gray-600">{audit.status}</p>
                      </div>
                      <span className="rounded-md bg-gray-100 px-2 py-1 text-sm font-medium text-gray-800">
                        Score {audit.score}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </>
        ) : (
          <p className="mt-6 text-sm text-red-600">{error || "Project not found"}</p>
        )}
      </div>
    </main>
  );
}
