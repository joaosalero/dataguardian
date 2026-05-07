"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8000";

type Project = {
  id: number;
  name: string;
  target: string;
  created_at: string;
};

type Audit = {
  id: number;
  project_id: number;
  status: string;
  summary: string;
  findings: string[];
  created_at: string;
};

type AnalysisListItem = {
  analysisId: number;
  projectId: number;
  inputType: "FILE" | "URL";
  status: string;
  riskLevel: "LOW" | "MEDIUM" | "HIGH";
  createdAt: string;
};

type AnalysisFinding = {
  id: number;
  code: string;
  title: string;
  description: string;
  severity: string;
  explanation?: string;
  recommendation?: string | null;
};

type MetadataEntry = {
  key: string;
  value: unknown;
  category: string;
  sensitivity: string;
  source: string;
  confidence: string;
};

type AnalysisDetail = {
  analysisId: number;
  projectId: number;
  inputType: "FILE" | "URL";
  status: string;
  summary: string;
  findings: AnalysisFinding[];
  metadata: {
    entries: MetadataEntry[];
  };
  riskScore: {
    score: number;
    level: "LOW" | "MEDIUM" | "HIGH";
  };
  cleanFile: null | {
    filename: string;
    mimeType: string;
    sizeBytes: number;
    checksumSha256: string;
    cleaningStatus: string;
    removedMetadataKeys: string[];
  };
  safePreview: null | {
    available: boolean;
    kind: string;
    mimeType?: string;
    dataUrl?: string;
    text?: string;
    message?: string;
  };
};

async function parseError(response: Response) {
  const text = await response.text();
  if (!text) {
    return "Request failed. Please try again.";
  }
  try {
    const parsed = JSON.parse(text) as { detail?: unknown };
    return typeof parsed.detail === "string" ? parsed.detail : "Request failed. Please try again.";
  } catch {
    return text;
  }
}

export default function DashboardPage() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [projects, setProjects] = useState<Project[]>([]);
  const [audits, setAudits] = useState<Audit[]>([]);
  const [analyses, setAnalyses] = useState<AnalysisListItem[]>([]);
  const [selectedAnalysis, setSelectedAnalysis] = useState<AnalysisDetail | null>(null);
  const [selectedProjectID, setSelectedProjectID] = useState<number | null>(null);
  const [projectName, setProjectName] = useState("");
  const [projectTarget, setProjectTarget] = useState("");
  const [analysisFile, setAnalysisFile] = useState<File | null>(null);
  const [analysisURL, setAnalysisURL] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [auditing, setAuditing] = useState(false);
  const [analyzingFile, setAnalyzingFile] = useState(false);
  const [analyzingURL, setAnalyzingURL] = useState(false);
  const [loadingAnalysis, setLoadingAnalysis] = useState(false);
  const [downloadingCleanFile, setDownloadingCleanFile] = useState(false);
  const [error, setError] = useState("");
  const [cleanFileError, setCleanFileError] = useState("");

  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectID) ?? null,
    [projects, selectedProjectID],
  );
  const visibleMetadataEntries = useMemo(
    () => selectedAnalysis?.metadata.entries.filter((entry) => entry.key !== "safe_preview_text") ?? [],
    [selectedAnalysis],
  );

  useEffect(() => {
    async function loadDashboard() {
      setError("");
      setLoading(true);

      try {
        const profileResponse = await apiFetch("/auth/me");
        if (profileResponse.status === 401) {
          router.push("/login");
          return;
        }
        if (!profileResponse.ok) {
          throw new Error(await parseError(profileResponse));
        }
        const profile = (await profileResponse.json()) as { email: string };
        setEmail(profile.email);

        const loadedProjects = await ensureDefaultProject(await fetchProjects());
        const firstProjectID = loadedProjects[0]?.id ?? null;
        setSelectedProjectID(firstProjectID);
        if (firstProjectID) {
          await fetchAudits(firstProjectID);
        } else {
          setAudits([]);
        }
        await fetchAnalyses();
      } catch (err) {
        setError(err instanceof Error ? err.message : "Could not load dashboard.");
      } finally {
        setLoading(false);
      }
    }

    loadDashboard();
  }, [router]);

  async function apiFetch(path: string, init?: RequestInit) {
    return fetch(`${API_BASE_URL}${path}`, {
      ...init,
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
        ...init?.headers,
      },
    });
  }

  async function fetchProjects() {
    const response = await apiFetch("/projects");
    if (response.status === 401) {
      router.push("/login");
      return [];
    }
    if (!response.ok) {
      throw new Error(await parseError(response));
    }
    const payload = (await response.json()) as { projects: Project[] };
    const loadedProjects = payload.projects ?? [];
    setProjects(loadedProjects);
    return loadedProjects;
  }

  async function ensureDefaultProject(loadedProjects: Project[]) {
    if (loadedProjects.length > 0) {
      return loadedProjects;
    }
    const response = await apiFetch("/projects", {
      method: "POST",
      body: JSON.stringify({
        name: "Default Project",
        target: "local-analysis",
      }),
    });
    if (!response.ok) {
      throw new Error(await parseError(response));
    }
    const project = (await response.json()) as Project;
    setProjects([project]);
    return [project];
  }

  async function fetchAudits(projectID: number) {
    const response = await apiFetch(`/projects/${projectID}/audits`);
    if (!response.ok) {
      throw new Error(await parseError(response));
    }
    const payload = (await response.json()) as { audits: Audit[] };
    setAudits(payload.audits ?? []);
  }

  async function fetchAnalyses() {
    const response = await apiFetch("/analyses");
    if (!response.ok) {
      throw new Error(await parseError(response));
    }
    const payload = (await response.json()) as { analyses: AnalysisListItem[] };
    setAnalyses(payload.analyses ?? []);
  }

  async function selectAnalysis(analysisID: number) {
    setError("");
    setCleanFileError("");
    setLoadingAnalysis(true);
    try {
      const response = await apiFetch(`/analyses/${analysisID}`);
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      setSelectedAnalysis((await response.json()) as AnalysisDetail);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load analysis details.");
    } finally {
      setLoadingAnalysis(false);
    }
  }

  async function createProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setSaving(true);

    try {
      const response = await apiFetch("/projects", {
        method: "POST",
        body: JSON.stringify({
          name: projectName,
          target: projectTarget,
        }),
      });
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      const project = (await response.json()) as Project;
      setProjects((current) => [project, ...current]);
      setSelectedProjectID(project.id);
      setAudits([]);
      setProjectName("");
      setProjectTarget("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not create project.");
    } finally {
      setSaving(false);
    }
  }

  async function downloadCleanFile() {
    if (!selectedAnalysis?.cleanFile) {
      setCleanFileError("No sanitized file is available for this analysis.");
      return;
    }
    setCleanFileError("");
    setDownloadingCleanFile(true);

    try {
      const response = await fetch(`${API_BASE_URL}/analyses/${selectedAnalysis.analysisId}/clean-file`, {
        credentials: "include",
      });
      if (response.status === 401) {
        router.push("/login");
        return;
      }
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      const blob = await response.blob();
      if (blob.size === 0) {
        throw new Error("The sanitized file response was empty.");
      }
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = selectedAnalysis.cleanFile.filename;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      setCleanFileError(err instanceof Error ? err.message : "Could not download sanitized file.");
    } finally {
      setDownloadingCleanFile(false);
    }
  }

  async function selectProject(projectID: number) {
    setError("");
    setSelectedProjectID(projectID);
    try {
      await fetchAudits(projectID);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load audits.");
    }
  }

  async function runAudit() {
    if (!selectedProjectID) {
      return;
    }
    setError("");
    setAuditing(true);

    try {
      const response = await apiFetch(`/projects/${selectedProjectID}/audit`, {
        method: "POST",
      });
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      const audit = (await response.json()) as Audit;
      setAudits((current) => [audit, ...current]);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not run audit.");
    } finally {
      setAuditing(false);
    }
  }

  async function analyzeFile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedProjectID || !analysisFile) {
      return;
    }
    setError("");
    setAnalyzingFile(true);

    try {
      const formData = new FormData();
      formData.append("projectId", String(selectedProjectID));
      formData.append("inputType", "FILE");
      formData.append("file", analysisFile);

      const response = await fetch(`${API_BASE_URL}/analyses`, {
        method: "POST",
        credentials: "include",
        body: formData,
      });
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      setAnalysisFile(null);
      await fetchAnalyses();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not analyze file.");
    } finally {
      setAnalyzingFile(false);
    }
  }

  async function analyzeURL(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selectedProjectID || !analysisURL.trim()) {
      return;
    }
    setError("");
    setAnalyzingURL(true);

    try {
      const response = await apiFetch("/analyses", {
        method: "POST",
        body: JSON.stringify({
          projectId: selectedProjectID,
          inputType: "URL",
          url: {
            originalUrl: analysisURL.trim(),
          },
        }),
      });
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      setAnalysisURL("");
      await fetchAnalyses();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not analyze URL.");
    } finally {
      setAnalyzingURL(false);
    }
  }

  async function logout() {
    await apiFetch("/auth/logout", { method: "POST" }).catch(() => undefined);
    router.push("/login");
  }

  return (
    <main className="min-h-screen px-6 py-8">
      <div className="mx-auto max-w-6xl">
        <header className="mb-8 flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
          <div>
            <h1 className="text-3xl font-semibold text-gray-950">DataGuardian</h1>
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

        {error ? (
          <p className="mb-5 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {error}
          </p>
        ) : null}

        <div className="grid gap-6">
          <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
            <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
              <div>
                <h2 className="text-base font-semibold text-gray-950">Projects</h2>
                <p className="mt-1 text-sm text-gray-600">
                  {loading ? "Loading projects." : `${projects.length} project${projects.length === 1 ? "" : "s"} tracked.`}
                </p>
              </div>
              <button
                className="rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
                disabled={!selectedProjectID || auditing}
                onClick={runAudit}
                type="button"
              >
                {auditing ? "Running audit..." : "Run audit"}
              </button>
            </div>

            <div className="mt-5 grid gap-3 md:grid-cols-2">
              {projects.map((project) => (
                <button
                  className={`rounded-lg border p-4 text-left transition hover:border-gray-400 ${
                    project.id === selectedProjectID
                      ? "border-gray-950 bg-gray-50"
                      : "border-gray-200 bg-white"
                  }`}
                  key={project.id}
                  onClick={() => selectProject(project.id)}
                  type="button"
                >
                  <span className="block text-sm font-semibold text-gray-950">{project.name}</span>
                  <span className="mt-1 block break-words text-sm text-gray-600">{project.target}</span>
                </button>
              ))}
            </div>

            {!loading && projects.length === 0 ? (
              <p className="mt-5 rounded-md bg-gray-50 px-4 py-3 text-sm text-gray-600">
                Preparing a default project for your analyses.
              </p>
            ) : null}

            <details className="mt-5 rounded-lg border border-gray-200 p-4">
              <summary className="cursor-pointer text-sm font-medium text-gray-800">
                Create another project
              </summary>
              <form className="mt-4 grid gap-4 lg:grid-cols-[1fr_1fr_auto]" onSubmit={createProject}>
                <label className="block">
                  <span className="text-sm font-medium text-gray-700">Project name</span>
                  <input
                    className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
                    disabled={saving}
                    onChange={(event) => setProjectName(event.target.value)}
                    required
                    value={projectName}
                  />
                </label>
                <label className="block">
                  <span className="text-sm font-medium text-gray-700">Database target</span>
                  <input
                    className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
                    disabled={saving}
                    onChange={(event) => setProjectTarget(event.target.value)}
                    placeholder="postgres://production-db"
                    required
                    value={projectTarget}
                  />
                </label>
                <div className="flex items-end">
                  <button
                    className="w-full rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400 lg:w-auto"
                    disabled={saving || !projectName.trim() || !projectTarget.trim()}
                    type="submit"
                  >
                    {saving ? "Creating..." : "Create project"}
                  </button>
                </div>
              </form>
            </details>
          </section>
        </div>

        <section className="mt-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
          <div>
            <h2 className="text-base font-semibold text-gray-950">Run Analysis</h2>
            <p className="mt-1 text-sm text-gray-600">
              {selectedProject
                ? `New analyses will be saved to ${selectedProject.name}.`
                : "Create or select a project before running an analysis."}
            </p>
          </div>

          <div className="mt-5 grid gap-5 lg:grid-cols-2">
            <form className="space-y-4" onSubmit={analyzeFile}>
              <label className="block">
                <span className="text-sm font-medium text-gray-700">Analysis file</span>
                <input
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
                  disabled={analyzingFile || !selectedProjectID}
                  onChange={(event) => setAnalysisFile(event.target.files?.[0] ?? null)}
                  type="file"
                />
              </label>
              <button
                className="rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
                disabled={analyzingFile || !selectedProjectID || !analysisFile}
                type="submit"
              >
                {analyzingFile ? "Analyzing file..." : "Analyze File"}
              </button>
            </form>

            <form className="space-y-4" onSubmit={analyzeURL}>
              <label className="block">
                <span className="text-sm font-medium text-gray-700">URL to analyze</span>
                <input
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
                  disabled={analyzingURL || !selectedProjectID}
                  onChange={(event) => setAnalysisURL(event.target.value)}
                  placeholder="https://example.com"
                  type="url"
                  value={analysisURL}
                />
              </label>
              <button
                className="rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
                disabled={analyzingURL || !selectedProjectID || !analysisURL.trim()}
                type="submit"
              >
                {analyzingURL ? "Analyzing URL..." : "Analyze URL"}
              </button>
            </form>
          </div>
        </section>

        <section className="mt-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
          <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
            <div>
              <h2 className="text-base font-semibold text-gray-950">Audit results</h2>
              <p className="mt-1 text-sm text-gray-600">
                {selectedProject
                  ? `Latest results for ${selectedProject.name}.`
                  : "Select or create a project to view audit results."}
              </p>
            </div>
          </div>

          <div className="mt-5 space-y-4">
            {audits.map((audit) => (
              <article className="rounded-lg border border-gray-200 p-4" key={audit.id}>
                <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
                  <p className="text-sm font-semibold text-gray-950">{audit.summary}</p>
                  <span className="w-fit rounded-full bg-green-50 px-3 py-1 text-xs font-medium uppercase text-green-700">
                    {audit.status}
                  </span>
                </div>
                <ul className="mt-3 list-disc space-y-1 pl-5 text-sm text-gray-600">
                  {audit.findings.map((finding) => (
                    <li key={finding}>{finding}</li>
                  ))}
                </ul>
              </article>
            ))}
          </div>

          {selectedProject && audits.length === 0 ? (
            <p className="mt-5 rounded-md bg-gray-50 px-4 py-3 text-sm text-gray-600">
              No audit results yet. Run an audit to generate the first baseline report.
            </p>
          ) : null}
        </section>

        <section className="mt-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
          <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
            <div>
              <h2 className="text-base font-semibold text-gray-950">Analysis history</h2>
              <p className="mt-1 text-sm text-gray-600">
                {loading ? "Loading analyses." : `${analyses.length} analysis run${analyses.length === 1 ? "" : "s"} available.`}
              </p>
            </div>
          </div>

          {analyses.length > 0 ? (
            <div className="mt-5 overflow-x-auto">
              <table className="w-full border-collapse text-left text-sm">
                <thead>
                  <tr className="border-b border-gray-200 text-xs uppercase text-gray-500">
                    <th className="py-2 pr-4 font-semibold">Type</th>
                    <th className="py-2 pr-4 font-semibold">Risk</th>
                    <th className="py-2 pr-4 font-semibold">Status</th>
                    <th className="py-2 pr-4 font-semibold">Created</th>
                    <th className="py-2 font-semibold">Action</th>
                  </tr>
                </thead>
                <tbody>
                  {analyses.map((analysis) => (
                    <tr className="border-b border-gray-100" key={analysis.analysisId}>
                      <td className="py-3 pr-4 font-medium text-gray-950">{analysis.inputType}</td>
                      <td className="py-3 pr-4">
                        <span className={`rounded-full px-2 py-1 text-xs font-semibold ${riskBadgeClass(analysis.riskLevel)}`}>
                          {analysis.riskLevel}
                        </span>
                      </td>
                      <td className="py-3 pr-4 text-gray-700">{analysis.status}</td>
                      <td className="py-3 pr-4 text-gray-600">{formatDate(analysis.createdAt)}</td>
                      <td className="py-3">
                        <button
                          className="rounded-md border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-800 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                          disabled={loadingAnalysis}
                          onClick={() => selectAnalysis(analysis.analysisId)}
                          type="button"
                        >
                          View
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : !loading ? (
            <p className="mt-5 rounded-md bg-gray-50 px-4 py-3 text-sm text-gray-600">
              No file or URL analyses have been created yet.
            </p>
          ) : null}
        </section>

        {selectedAnalysis ? (
          <section className="mt-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
            <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
              <div>
                <h2 className="text-base font-semibold text-gray-950">Analysis details</h2>
                <p className="mt-1 text-sm text-gray-600">{selectedAnalysis.summary}</p>
              </div>
              <span className={`w-fit rounded-full px-3 py-1 text-xs font-semibold ${riskBadgeClass(selectedAnalysis.riskScore.level)}`}>
                {selectedAnalysis.riskScore.level} risk · {selectedAnalysis.riskScore.score}
              </span>
            </div>

            <div className="mt-5 grid gap-5 lg:grid-cols-2">
              <div>
                <h3 className="text-sm font-semibold text-gray-950">Findings</h3>
                {selectedAnalysis.findings.length > 0 ? (
                  <div className="mt-3 space-y-3">
                    {selectedAnalysis.findings.map((finding) => (
                      <article className="rounded-lg border border-gray-200 p-3" key={`${finding.id}-${finding.code}`}>
                        <div className="flex items-center justify-between gap-3">
                          <p className="text-sm font-semibold text-gray-950">{finding.title}</p>
                          <span className="text-xs font-medium text-gray-500">{finding.severity}</span>
                        </div>
                        <p className="mt-1 text-xs text-gray-500">{finding.code}</p>
                        <p className="mt-2 text-sm text-gray-600">{finding.description}</p>
                        {finding.explanation ? (
                          <p className="mt-3 text-sm text-gray-700">{finding.explanation}</p>
                        ) : null}
                        {finding.recommendation ? (
                          <p className="mt-2 text-sm font-medium text-gray-800">
                            Mitigation: {finding.recommendation}
                          </p>
                        ) : null}
                      </article>
                    ))}
                  </div>
                ) : (
                  <p className="mt-3 rounded-md bg-gray-50 px-4 py-3 text-sm text-gray-600">
                    No findings were recorded for this analysis.
                  </p>
                )}
              </div>

              <div>
                <h3 className="text-sm font-semibold text-gray-950">Metadata</h3>
                {visibleMetadataEntries.length > 0 ? (
                  <dl className="mt-3 divide-y divide-gray-100 rounded-lg border border-gray-200">
                    {visibleMetadataEntries.map((entry) => (
                      <div className="grid gap-1 p-3 sm:grid-cols-[150px_1fr]" key={`${entry.key}-${entry.source}`}>
                        <dt className="text-xs font-semibold uppercase text-gray-500">{entry.key}</dt>
                        <dd className="break-words text-sm text-gray-800">
                          {formatMetadataValue(entry.value)}
                          <span className="mt-1 block text-xs text-gray-500">
                            {entry.category} · {entry.sensitivity}
                          </span>
                        </dd>
                      </div>
                    ))}
                  </dl>
                ) : (
                  <p className="mt-3 rounded-md bg-gray-50 px-4 py-3 text-sm text-gray-600">
                    No metadata entries were recorded for this analysis.
                  </p>
                )}
              </div>
            </div>

            <div className="mt-5 rounded-lg border border-gray-200 p-4">
              <h3 className="text-sm font-semibold text-gray-950">Safe Preview</h3>
              <p className="mt-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                This preview is rendered in isolated safe mode. Active content is not executed.
              </p>
              {selectedAnalysis.riskScore.level === "HIGH" ? (
                <p className="mt-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                  This content may be unsafe. Do not trust or open locally without caution.
                </p>
              ) : null}
              {selectedAnalysis.safePreview?.available ? (
                <div className="mt-3">
                  {selectedAnalysis.safePreview.kind === "image" && selectedAnalysis.safePreview.dataUrl ? (
                    <img
                      alt="Static safe preview"
                      className="max-h-[520px] max-w-full rounded-md border border-gray-200 bg-gray-50 object-contain"
                      src={selectedAnalysis.safePreview.dataUrl}
                    />
                  ) : null}
                  {selectedAnalysis.safePreview.kind === "text" && selectedAnalysis.safePreview.text ? (
                    <pre className="max-h-80 overflow-auto rounded-md border border-gray-200 bg-gray-50 p-3 text-sm text-gray-800 whitespace-pre-wrap">
                      {selectedAnalysis.safePreview.text}
                    </pre>
                  ) : null}
                </div>
              ) : (
                <p className="mt-3 rounded-md bg-gray-50 px-4 py-3 text-sm text-gray-600">
                  {selectedAnalysis.safePreview?.message ?? "No safe preview is available for this analysis."}
                </p>
              )}
            </div>

            <div className="mt-5 rounded-lg border border-gray-200 p-4">
              <h3 className="text-sm font-semibold text-gray-950">Sanitized File</h3>
              {selectedAnalysis.cleanFile ? (
                <div className="mt-3 space-y-3 text-sm text-gray-700">
                  <dl className="grid gap-2 sm:grid-cols-3">
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">Filename</dt>
                      <dd className="mt-1 break-words text-gray-900">{selectedAnalysis.cleanFile.filename}</dd>
                    </div>
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">Size</dt>
                      <dd className="mt-1 text-gray-900">{formatBytes(selectedAnalysis.cleanFile.sizeBytes)}</dd>
                    </div>
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">Cleaning status</dt>
                      <dd className="mt-1 text-gray-900">{selectedAnalysis.cleanFile.cleaningStatus}</dd>
                    </div>
                  </dl>
                  <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                    This version has metadata removed. It does NOT guarantee the file is safe.
                  </p>
                  <p className="text-xs text-gray-500">
                    Removed metadata: {selectedAnalysis.cleanFile.removedMetadataKeys.length > 0 ? selectedAnalysis.cleanFile.removedMetadataKeys.join(", ") : "none recorded"}
                  </p>
                  {cleanFileError ? (
                    <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                      {cleanFileError}
                    </p>
                  ) : null}
                  <button
                    className="rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
                    disabled={downloadingCleanFile || selectedAnalysis.cleanFile.cleaningStatus !== "COMPLETED"}
                    onClick={downloadCleanFile}
                    type="button"
                  >
                    {downloadingCleanFile ? "Downloading..." : "Download Clean File"}
                  </button>
                </div>
              ) : (
                <p className="mt-2 text-sm text-gray-600">No clean file is available for this analysis.</p>
              )}
            </div>
          </section>
        ) : null}
      </div>
    </main>
  );
}

function riskBadgeClass(level: string) {
  if (level === "HIGH") {
    return "bg-red-50 text-red-700";
  }
  if (level === "MEDIUM") {
    return "bg-amber-50 text-amber-700";
  }
  return "bg-green-50 text-green-700";
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(new Date(value));
}

function formatMetadataValue(value: unknown) {
  if (value === null || value === undefined) {
    return "Not recorded";
  }
  if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return JSON.stringify(value);
}

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value < 0) {
    return "Not recorded";
  }
  if (value < 1024) {
    return `${value} B`;
  }
  if (value < 1024 * 1024) {
    return `${(value / 1024).toFixed(1)} KB`;
  }
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}
