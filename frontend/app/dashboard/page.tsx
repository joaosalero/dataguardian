"use client";

import { FormEvent, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { LanguageSwitcher, useI18n } from "../i18n";

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

type AnalysisPagination = {
  page: number;
  pageSize: number;
  totalItems: number;
  totalPages: number;
  hasNext: boolean;
  hasPrevious: boolean;
};

type AnalysisFilters = {
  inputType: "" | "FILE" | "URL";
  riskLevel: "" | "LOW" | "MEDIUM" | "HIGH";
  status: "" | "PENDING" | "PROCESSING" | "COMPLETED" | "FAILED";
};

type StorageSummary = {
  fileCount: number;
  totalBytes: number;
  orphanRetentionHours: number;
};

type UserProfile = {
  email: string;
  isAdmin?: boolean;
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
  file?: null | {
    originalFilename: string;
    mimeType: string;
    sizeBytes: number;
    checksumSha256: string;
  };
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
  const { locale, t } = useI18n();
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [projects, setProjects] = useState<Project[]>([]);
  const [audits, setAudits] = useState<Audit[]>([]);
  const [analyses, setAnalyses] = useState<AnalysisListItem[]>([]);
  const [analysisPagination, setAnalysisPagination] = useState<AnalysisPagination>({
    page: 1,
    pageSize: 10,
    totalItems: 0,
    totalPages: 0,
    hasNext: false,
    hasPrevious: false,
  });
  const [analysisFilters, setAnalysisFilters] = useState<AnalysisFilters>({
    inputType: "",
    riskLevel: "",
    status: "",
  });
  const [storageSummary, setStorageSummary] = useState<StorageSummary | null>(null);
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
  const [deletingAnalysisID, setDeletingAnalysisID] = useState<number | null>(null);
  const [downloadingOriginalFile, setDownloadingOriginalFile] = useState(false);
  const [downloadingCleanFile, setDownloadingCleanFile] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [originalFileError, setOriginalFileError] = useState("");
  const [cleanFileError, setCleanFileError] = useState("");
  const [isAdmin, setIsAdmin] = useState(false);
  const [previewImageFailed, setPreviewImageFailed] = useState(false);
  const [darkMode, setDarkMode] = useState(false);

  const selectedProject = useMemo(
    () => projects.find((project) => project.id === selectedProjectID) ?? null,
    [projects, selectedProjectID],
  );
  const visibleMetadataEntries = useMemo(
    () => selectedAnalysis?.metadata.entries.filter((entry) => entry.key !== "safe_preview_text") ?? [],
    [selectedAnalysis],
  );
  const hasAnalysisFilters = analysisFilters.inputType !== "" || analysisFilters.riskLevel !== "" || analysisFilters.status !== "";

  function redirectToExpiredLogin() {
    router.push("/login?reason=session-expired");
  }

  function handleExpiredSession(response: Response) {
    if (response.status !== 401) {
      return false;
    }
    redirectToExpiredLogin();
    return true;
  }

  useEffect(() => {
    async function loadDashboard() {
      setError("");
      setNotice("");
      setLoading(true);

      try {
        const profileResponse = await apiFetch("/auth/me");
        if (profileResponse.status === 401) {
          redirectToExpiredLogin();
          return;
        }
        if (!profileResponse.ok) {
          throw new Error(await parseError(profileResponse));
        }
        const profile = (await profileResponse.json()) as UserProfile;
        setEmail(profile.email);
        setIsAdmin(Boolean(profile.isAdmin));

        const loadedProjects = await ensureDefaultProject(await fetchProjects());
        const firstProjectID = loadedProjects[0]?.id ?? null;
        setSelectedProjectID(firstProjectID);
        if (firstProjectID) {
          await fetchAudits(firstProjectID);
        } else {
          setAudits([]);
        }
        await fetchAnalyses({ page: 1 });
        const requestedAnalysis = Number(new URLSearchParams(window.location.search).get("analysis"));
        if (Number.isSafeInteger(requestedAnalysis) && requestedAnalysis > 0) {
          await selectAnalysis(requestedAnalysis);
        }
        if (profile.isAdmin) {
          await fetchStorageSummary();
        }
      } catch (err) {
        setError(err instanceof Error ? err.message : "Could not load dashboard.");
      } finally {
        setLoading(false);
      }
    }

    loadDashboard();
  }, [router]);

  useEffect(() => {
    setPreviewImageFailed(false);
  }, [selectedAnalysis?.analysisId, selectedAnalysis?.safePreview?.kind, selectedAnalysis?.safePreview?.dataUrl]);

  useEffect(() => {
    const enabled = window.localStorage.getItem("dataguardian-theme") === "dark";
    setDarkMode(enabled);
    document.documentElement.classList.toggle("dark", enabled);
  }, []);

  function toggleDarkMode() {
    const enabled = !darkMode;
    setDarkMode(enabled);
    window.localStorage.setItem("dataguardian-theme", enabled ? "dark" : "light");
    document.documentElement.classList.toggle("dark", enabled);
  }

  function exportSelectedAnalysis(format: "json" | "pdf") {
    if (!selectedAnalysis) return;
    if (format === "json") {
      downloadGeneratedFile(`dataguardian-analysis-${selectedAnalysis.analysisId}.json`, "application/json", JSON.stringify(selectedAnalysis, null, 2));
      return;
    }
      downloadGeneratedFile(`dataguardian-analysis-${selectedAnalysis.analysisId}.pdf`, "application/pdf", buildStaticPDF(selectedAnalysis, locale));
  }

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
    if (handleExpiredSession(response)) {
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
    if (handleExpiredSession(response)) {
      return [];
    }
    if (!response.ok) {
      throw new Error(await parseError(response));
    }
    const project = (await response.json()) as Project;
    setProjects([project]);
    return [project];
  }

  async function fetchAudits(projectID: number) {
    const response = await apiFetch(`/projects/${projectID}/audits`);
    if (handleExpiredSession(response)) {
      return;
    }
    if (!response.ok) {
      throw new Error(await parseError(response));
    }
    const payload = (await response.json()) as { audits: Audit[] };
    setAudits(payload.audits ?? []);
  }

  async function fetchAnalyses(options?: { page?: number; filters?: AnalysisFilters }) {
    const page = options?.page ?? analysisPagination.page;
    const filters = options?.filters ?? analysisFilters;
    const query = new URLSearchParams({
      page: String(page),
      pageSize: String(analysisPagination.pageSize),
    });
    if (filters.inputType) {
      query.set("inputType", filters.inputType);
    }
    if (filters.riskLevel) {
      query.set("riskLevel", filters.riskLevel);
    }
    if (filters.status) {
      query.set("status", filters.status);
    }
    const response = await apiFetch(`/analyses?${query.toString()}`);
    if (handleExpiredSession(response)) {
      return;
    }
    if (!response.ok) {
      throw new Error(await parseError(response));
    }
    const payload = (await response.json()) as { analyses: AnalysisListItem[]; pagination?: AnalysisPagination };
    setAnalyses(payload.analyses ?? []);
    if (payload.pagination) {
      setAnalysisPagination(payload.pagination);
    }
  }

  async function fetchStorageSummary() {
    const response = await apiFetch("/storage");
    if (handleExpiredSession(response)) {
      return;
    }
    if (response.status === 403 || !response.ok) {
      setStorageSummary(null);
      return;
    }
    setStorageSummary((await response.json()) as StorageSummary);
  }

  async function selectAnalysis(analysisID: number) {
    setError("");
    setOriginalFileError("");
    setCleanFileError("");
    setLoadingAnalysis(true);
    try {
      const response = await apiFetch(`/analyses/${analysisID}`);
      if (handleExpiredSession(response)) {
        return;
      }
      if (response.status === 404) {
        setSelectedAnalysis(null);
        await fetchAnalyses();
        throw new Error("This analysis is no longer available.");
      }
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      setSelectedAnalysis((await response.json()) as AnalysisDetail);
      window.history.replaceState(null, "", `/dashboard?analysis=${analysisID}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load analysis details.");
    } finally {
      setLoadingAnalysis(false);
    }
  }

  async function createProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError("");
    setNotice("");
    setSaving(true);

    try {
      const response = await apiFetch("/projects", {
        method: "POST",
        body: JSON.stringify({
          name: projectName,
          target: projectTarget,
        }),
      });
      if (handleExpiredSession(response)) {
        return;
      }
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      const project = (await response.json()) as Project;
      setProjects((current) => [project, ...current]);
      setSelectedProjectID(project.id);
      setAudits([]);
      setProjectName("");
      setProjectTarget("");
      setNotice("Project created.");
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
    if (selectedAnalysis.cleanFile.cleaningStatus !== "COMPLETED") {
      setCleanFileError("The sanitized file is not available because cleaning did not complete.");
      return;
    }
    setCleanFileError("");
    setDownloadingCleanFile(true);

    try {
      const response = await fetch(`${API_BASE_URL}/analyses/${selectedAnalysis.analysisId}/clean-file`, {
        credentials: "include",
      });
      if (handleExpiredSession(response)) {
        return;
      }
      if (response.status === 404) {
        throw new Error("The sanitized file is no longer available for this analysis.");
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

  async function downloadOriginalFile() {
    if (!selectedAnalysis?.file) {
      setOriginalFileError("No original file is available for this analysis.");
      return;
    }
    if (selectedAnalysis.riskScore.level === "HIGH" && !confirmHighRiskOriginalDownload(selectedAnalysis)) {
      return;
    }
    setOriginalFileError("");
    setDownloadingOriginalFile(true);

    try {
      const response = await fetch(`${API_BASE_URL}/analyses/${selectedAnalysis.analysisId}/file`, {
        credentials: "include",
      });
      if (handleExpiredSession(response)) {
        return;
      }
      if (response.status === 404) {
        throw new Error("The original file is no longer available for this analysis.");
      }
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      const blob = await response.blob();
      if (blob.size === 0) {
        throw new Error("The original file response was empty.");
      }
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = selectedAnalysis.file.originalFilename;
      document.body.appendChild(anchor);
      anchor.click();
      anchor.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      setOriginalFileError(err instanceof Error ? err.message : "Could not download original file.");
    } finally {
      setDownloadingOriginalFile(false);
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
    setNotice("");
    setAuditing(true);

    try {
      const response = await apiFetch(`/projects/${selectedProjectID}/audit`, {
        method: "POST",
      });
      if (handleExpiredSession(response)) {
        return;
      }
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      const audit = (await response.json()) as Audit;
      setAudits((current) => [audit, ...current]);
      setNotice("Audit completed.");
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
    setNotice("");
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
      if (handleExpiredSession(response)) {
        return;
      }
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      setAnalysisFile(null);
      await fetchAnalyses({ page: 1 });
      if (isAdmin) {
        await fetchStorageSummary();
      }
      setNotice("File analysis completed.");
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
    setNotice("");
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
      if (handleExpiredSession(response)) {
        return;
      }
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      setAnalysisURL("");
      await fetchAnalyses({ page: 1 });
      if (isAdmin) {
        await fetchStorageSummary();
      }
      setNotice("URL analysis completed.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not analyze URL.");
    } finally {
      setAnalyzingURL(false);
    }
  }

  async function updateAnalysisFilters(nextFilters: AnalysisFilters) {
    setAnalysisFilters(nextFilters);
    setError("");
    setNotice("");
    try {
      await fetchAnalyses({ page: 1, filters: nextFilters });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load analyses.");
    }
  }

  async function changeAnalysisPage(page: number) {
    setError("");
    setNotice("");
    try {
      await fetchAnalyses({ page });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load analyses.");
    }
  }

  async function deleteAnalysis(analysisID: number) {
    if (!window.confirm("Delete this analysis and its stored files? This cannot be undone.")) {
      return;
    }
    setError("");
    setNotice("");
    setDeletingAnalysisID(analysisID);
    try {
      const response = await apiFetch(`/analyses/${analysisID}`, { method: "DELETE" });
      if (handleExpiredSession(response)) {
        return;
      }
      if (!response.ok) {
        throw new Error(await parseError(response));
      }
      if (selectedAnalysis?.analysisId === analysisID) {
        setSelectedAnalysis(null);
      }
      const nextPage =
        analyses.length === 1 && analysisPagination.page > 1
          ? analysisPagination.page - 1
          : analysisPagination.page;
      await fetchAnalyses({ page: nextPage });
      if (isAdmin) {
        await fetchStorageSummary();
      }
      setNotice("Analysis deleted.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not delete analysis.");
    } finally {
      setDeletingAnalysisID(null);
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
              {email ? (locale === "pt-BR" ? `Conectado como ${email}.` : `Signed in as ${email}.`) : (locale === "pt-BR" ? "Verificando sua sessão." : "Verifying your session.")}
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
          <LanguageSwitcher />
          <Link className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-800 hover:bg-gray-50" href="/profile">{t("dashboard.profile", "Profile")}</Link>
          <button className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-800 hover:bg-gray-50" onClick={toggleDarkMode} type="button">
            {darkMode ? t("dashboard.lightMode", "Light mode") : t("dashboard.darkMode", "Dark mode")}
          </button>
          <button
            className="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-800 hover:bg-gray-50"
            onClick={logout}
            type="button"
          >
            {t("auth.signOut", "Sign out")}
          </button>
          </div>
        </header>

        <nav aria-label="Primary" className="mb-5 flex flex-wrap gap-2 text-sm">
          <a className="rounded-md bg-gray-950 px-3 py-2 font-medium text-white" href="#analyses">{t("dashboard.analyses", "Analyses")}</a>
          <a className="rounded-md border border-gray-300 px-3 py-2 font-medium" href="#projects">{t("dashboard.projects", "Projects")}</a>
          <Link className="rounded-md border border-gray-300 px-3 py-2 font-medium" href="/profile">{t("dashboard.settings", "Settings")}</Link>
        </nav>

        {error ? (
          <p aria-live="assertive" className="mb-5 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700" role="alert">
            {error}
          </p>
        ) : null}
        {notice ? (
          <p aria-live="polite" className="mb-5 rounded-md border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-700" role="status">
            {notice}
          </p>
        ) : null}

        <div className="grid gap-6">
          <section className="rounded-lg border border-gray-200 bg-white p-5 shadow-sm" id="projects">
            <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
              <div>
                <h2 className="text-base font-semibold text-gray-950">{t("dashboard.projects", "Projects")}</h2>
                <p className="mt-1 text-sm text-gray-600">
                  {loading ? (locale === "pt-BR" ? "Carregando projetos." : "Loading projects.") : locale === "pt-BR" ? `${projects.length} projeto${projects.length === 1 ? "" : "s"} acompanhado${projects.length === 1 ? "" : "s"}.` : `${projects.length} project${projects.length === 1 ? "" : "s"} tracked.`}
                </p>
              </div>
              <button
                className="rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
                disabled={!selectedProjectID || auditing}
                onClick={runAudit}
                type="button"
              >
                {auditing ? (locale === "pt-BR" ? "Executando checklist..." : "Running checklist...") : (locale === "pt-BR" ? "Executar checklist de prontidão" : "Run readiness checklist")}
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
                  <span className="text-sm font-medium text-gray-700">{locale === "pt-BR" ? "Nome do projeto" : "Project name"}</span>
                  <input
                    className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
                    disabled={saving}
                    onChange={(event) => setProjectName(event.target.value)}
                    required
                    value={projectName}
                    maxLength={120}
                  />
                </label>
                <label className="block">
                  <span className="text-sm font-medium text-gray-700">{locale === "pt-BR" ? "Destino do banco de dados" : "Database target"}</span>
                  <input
                    className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
                    disabled={saving}
                    onChange={(event) => setProjectTarget(event.target.value)}
                    placeholder="postgres://production-db"
                    required
                    value={projectTarget}
                    maxLength={2048}
                  />
                </label>
                <div className="flex items-end">
                  <button
                    className="w-full rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400 lg:w-auto"
                    disabled={saving || !projectName.trim() || !projectTarget.trim()}
                    type="submit"
                  >
                    {saving ? (locale === "pt-BR" ? "Criando..." : "Creating...") : (locale === "pt-BR" ? "Criar projeto" : "Create project")}
                  </button>
                </div>
              </form>
            </details>
          </section>
        </div>

        <section className="mt-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
          <div>
            <h2 className="text-base font-semibold text-gray-950">{t("dashboard.runAnalysis", "Run Analysis")}</h2>
            <p className="mt-1 text-sm text-gray-600">
              {selectedProject
                ? `New analyses will be saved to ${selectedProject.name}.`
                : locale === "pt-BR" ? "Crie ou selecione um projeto antes de executar uma análise." : "Create or select a project before running an analysis."}
            </p>
          </div>

          <div className="mt-5 grid gap-5 lg:grid-cols-2">
            <form className="space-y-4" onSubmit={analyzeFile}>
              <label className="block">
                <span className="text-sm font-medium text-gray-700">{locale === "pt-BR" ? "Arquivo para análise" : "Analysis file"}</span>
                <input
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
                  disabled={analyzingFile || !selectedProjectID}
                  onChange={(event) => setAnalysisFile(event.target.files?.[0] ?? null)}
                  type="file"
                  accept=".pdf,.jpg,.jpeg,.png,.txt,application/pdf,image/jpeg,image/png,text/plain"
                />
              </label>
              <button
                className="rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
                disabled={analyzingFile || !selectedProjectID || !analysisFile}
                type="submit"
              >
                {analyzingFile ? t("dashboard.analyzingFile", "Analyzing file...") : t("dashboard.analyzeFile", "Analyze File")}
              </button>
            </form>

            <form className="space-y-4" onSubmit={analyzeURL}>
              <label className="block">
                <span className="text-sm font-medium text-gray-700">{locale === "pt-BR" ? "URL para análise" : "URL to analyze"}</span>
                <input
                  className="mt-1 w-full rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
                  disabled={analyzingURL || !selectedProjectID}
                  onChange={(event) => setAnalysisURL(event.target.value)}
                  placeholder="https://example.com"
                  type="url"
                  maxLength={2048}
                  value={analysisURL}
                />
              </label>
              <button
                className="rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
                disabled={analyzingURL || !selectedProjectID || !analysisURL.trim()}
                type="submit"
              >
                {analyzingURL ? t("dashboard.analyzingUrl", "Analyzing URL...") : t("dashboard.analyzeUrl", "Analyze URL")}
              </button>
            </form>
          </div>
        </section>

        <section className="mt-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm" id="analyses">
          <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
            <div>
              <h2 className="text-base font-semibold text-gray-950">{locale === "pt-BR" ? "Checklists de prontidão do projeto" : "Project readiness checklists"}</h2>
              <p className="mt-1 text-sm text-gray-600">
                {selectedProject
                  ? `Latest results for ${selectedProject.name}.`
                  : locale === "pt-BR" ? "Selecione ou crie um projeto para visualizar os resultados da auditoria." : "Select or create a project to view audit results."}
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
              {locale === "pt-BR" ? "Ainda não há resultados de auditoria. Execute uma auditoria para gerar o primeiro relatório de referência." : "No audit results yet. Run an audit to generate the first baseline report."}
            </p>
          ) : null}
        </section>

        <section className="mt-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
          <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
            <div>
              <h2 className="text-base font-semibold text-gray-950">{t("dashboard.analysisHistory", "Analysis history")}</h2>
              <p className="mt-1 text-sm text-gray-600">
                {loading
                  ? (locale === "pt-BR" ? "Carregando análises." : "Loading analyses.")
                  : locale === "pt-BR" ? `${analysisPagination.totalItems} análise${analysisPagination.totalItems === 1 ? "" : "s"} correspondente${analysisPagination.totalItems === 1 ? "" : "s"}.` : `${analysisPagination.totalItems} matching analysis run${analysisPagination.totalItems === 1 ? "" : "s"}.`}
              </p>
            </div>
          </div>

          <div className="mt-5 grid gap-3 md:grid-cols-4">
            <label className="block">
              <span className="text-xs font-semibold uppercase text-gray-500">{t("dashboard.type", "Type")}</span>
              <select
                className="mt-1 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm outline-none focus:border-gray-900"
                onChange={(event) => updateAnalysisFilters({ ...analysisFilters, inputType: event.target.value as AnalysisFilters["inputType"] })}
                value={analysisFilters.inputType}
              >
                <option value="">{locale === "pt-BR" ? "Todos os tipos" : "All types"}</option>
                <option value="FILE">{locale === "pt-BR" ? "Arquivos" : "Files"}</option>
                <option value="URL">URLs</option>
              </select>
            </label>
            <label className="block">
              <span className="text-xs font-semibold uppercase text-gray-500">{t("dashboard.risk", "Risk")}</span>
              <select
                className="mt-1 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm outline-none focus:border-gray-900"
                onChange={(event) => updateAnalysisFilters({ ...analysisFilters, riskLevel: event.target.value as AnalysisFilters["riskLevel"] })}
                value={analysisFilters.riskLevel}
              >
                <option value="">{locale === "pt-BR" ? "Todos os riscos" : "All risks"}</option>
                <option value="LOW">{localizedEnum("LOW", locale)}</option>
                <option value="MEDIUM">{localizedEnum("MEDIUM", locale)}</option>
                <option value="HIGH">{localizedEnum("HIGH", locale)}</option>
              </select>
            </label>
            <label className="block">
              <span className="text-xs font-semibold uppercase text-gray-500">{t("dashboard.status", "Status")}</span>
              <select
                className="mt-1 w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm outline-none focus:border-gray-900"
                onChange={(event) => updateAnalysisFilters({ ...analysisFilters, status: event.target.value as AnalysisFilters["status"] })}
                value={analysisFilters.status}
              >
                <option value="">{locale === "pt-BR" ? "Todos os status" : "All statuses"}</option>
                {(["COMPLETED", "PROCESSING", "FAILED", "PENDING"] as const).map((status) => <option key={status} value={status}>{localizedEnum(status, locale)}</option>)}
              </select>
            </label>
            {isAdmin ? (
              <div className="rounded-md bg-gray-50 px-3 py-2 text-sm text-gray-600">
                <span className="block text-xs font-semibold uppercase text-gray-500">{t("dashboard.storage", "Storage")}</span>
                {storageSummary
                  ? `${storageSummary.fileCount} file${storageSummary.fileCount === 1 ? "" : "s"} - ${formatBytes(storageSummary.totalBytes)}`
                  : t("common.notAvailable", "Not available")}
              </div>
            ) : null}
          </div>

          {analyses.length > 0 ? (
            <div className="mt-5 overflow-x-auto">
              <table className="w-full border-collapse text-left text-sm">
                <thead>
                  <tr className="border-b border-gray-200 text-xs uppercase text-gray-500">
                    <th className="py-2 pr-4 font-semibold">{t("dashboard.type", "Type")}</th>
                    <th className="py-2 pr-4 font-semibold">{t("dashboard.risk", "Risk")}</th>
                    <th className="py-2 pr-4 font-semibold">{t("dashboard.status", "Status")}</th>
                    <th className="py-2 pr-4 font-semibold">{locale === "pt-BR" ? "Criada em" : "Created"}</th>
                    <th className="py-2 font-semibold">{locale === "pt-BR" ? "Ações" : "Actions"}</th>
                  </tr>
                </thead>
                <tbody>
                  {analyses.map((analysis) => (
                    <tr className="border-b border-gray-100" key={analysis.analysisId}>
                      <td className="py-3 pr-4 font-medium text-gray-950">{analysis.inputType}</td>
                      <td className="py-3 pr-4">
                        <span className={`rounded-full px-2 py-1 text-xs font-semibold ${riskBadgeClass(analysis.riskLevel)}`}>
                          {localizedEnum(analysis.riskLevel, locale)}
                        </span>
                      </td>
                      <td className="py-3 pr-4 text-gray-700">{localizedEnum(analysis.status, locale)}</td>
                      <td className="py-3 pr-4 text-gray-600">{formatDate(analysis.createdAt, locale)}</td>
                      <td className="py-3">
                        <div className="flex flex-wrap gap-2">
                          <button
                            className="rounded-md border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-800 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                            disabled={loadingAnalysis}
                            onClick={() => selectAnalysis(analysis.analysisId)}
                            type="button"
                          >
                            {t("dashboard.view", "View")}
                          </button>
                          <button
                            className="rounded-md border border-red-200 px-3 py-1.5 text-xs font-medium text-red-700 hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-60"
                            disabled={deletingAnalysisID === analysis.analysisId}
                            onClick={() => deleteAnalysis(analysis.analysisId)}
                            type="button"
                          >
                            {deletingAnalysisID === analysis.analysisId ? (locale === "pt-BR" ? "Excluindo..." : "Deleting...") : t("dashboard.delete", "Delete")}
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : !loading ? (
            <p className="mt-5 rounded-md bg-gray-50 px-4 py-3 text-sm text-gray-600">
              {analysisPagination.totalItems > 0
                ? (locale === "pt-BR" ? "Nenhuma análise é exibida nesta página. Use Anterior ou ajuste os filtros." : "No analyses are shown on this page. Use Previous or adjust the filters.")
                : hasAnalysisFilters
                  ? (locale === "pt-BR" ? "Nenhuma análise corresponde aos filtros atuais. Tente limpar um filtro." : "No analyses match the current filters. Try clearing one filter.")
                  : (locale === "pt-BR" ? "Ainda não há análises. Envie um arquivo ou uma URL para criar o primeiro resultado." : "No analyses yet. Upload a file or submit a URL to create the first result.")}
            </p>
          ) : null}

          {analysisPagination.totalPages > 0 ? (
            <div className="mt-5 flex flex-col justify-between gap-3 sm:flex-row sm:items-center">
              <p className="text-sm text-gray-600">
                {locale === "pt-BR" ? `Página ${analysisPagination.page} de ${analysisPagination.totalPages}` : `Page ${analysisPagination.page} of ${analysisPagination.totalPages}`}
              </p>
              <div className="flex gap-2">
                <button
                  className="rounded-md border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-800 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={!analysisPagination.hasPrevious}
                  onClick={() => changeAnalysisPage(analysisPagination.page - 1)}
                  type="button"
                >
                  {t("dashboard.previous", "Previous")}
                </button>
                <button
                  className="rounded-md border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-800 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60"
                  disabled={!analysisPagination.hasNext}
                  onClick={() => changeAnalysisPage(analysisPagination.page + 1)}
                  type="button"
                >
                  {t("dashboard.next", "Next")}
                </button>
              </div>
            </div>
          ) : null}
        </section>

        {selectedAnalysis ? (
          <section className="mt-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
            <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
              <div>
                <h2 className="text-base font-semibold text-gray-950">{t("dashboard.analysisDetails", "Analysis details")}</h2>
                <p className="mt-1 text-sm text-gray-600">{localizedSummary(selectedAnalysis.summary, locale)}</p>
              </div>
              <div className="flex flex-wrap items-center gap-2">
              <button className="rounded-md border border-gray-300 px-3 py-1 text-xs font-medium" onClick={() => exportSelectedAnalysis("json")} type="button">{t("dashboard.exportJson", "Export JSON")}</button>
              <button className="rounded-md border border-gray-300 px-3 py-1 text-xs font-medium" onClick={() => exportSelectedAnalysis("pdf")} type="button">{t("dashboard.exportPdf", "Export static PDF")}</button>
              <button className="rounded-md border border-gray-300 px-3 py-1 text-xs font-medium" onClick={() => navigator.clipboard.writeText(`${window.location.origin}/analyses/${selectedAnalysis.analysisId}`).then(() => setNotice(t("dashboard.linkCopied", "Analysis link copied.")), () => setError("Could not copy the analysis link."))} type="button">{t("dashboard.copyLink", "Copy link")}</button>
              <span className={`w-fit rounded-full px-3 py-1 text-xs font-semibold ${riskBadgeClass(selectedAnalysis.riskScore.level)}`}>
                {localizedEnum(selectedAnalysis.riskScore.level, locale)} {locale === "pt-BR" ? "risco" : "risk"} · {selectedAnalysis.riskScore.score}
              </span>
              </div>
            </div>

            <ol className="mt-5 grid gap-2 text-xs font-semibold uppercase text-gray-500 sm:grid-cols-5">
              {(locale === "pt-BR" ? ["Inspecionar", "Revisar risco", "Revisar achados", "Revisar prévia", "Decidir download"] : ["Inspect", "Review Risk", "Review Findings", "Review Preview", "Decide Download"]).map((step, index) => (
                <li className="rounded-md border border-gray-200 bg-gray-50 px-3 py-2" key={step}>
                  <span className="mr-2 text-gray-400">{index + 1}</span>
                  {step}
                </li>
              ))}
            </ol>

            <div className="mt-5 grid gap-5 lg:grid-cols-2">
              <div>
                <h3 className="text-sm font-semibold text-gray-950">{t("dashboard.findings", "Review Findings")}</h3>
                {selectedAnalysis.findings.length > 0 ? (
                  <div className="mt-3 space-y-3">
                    {selectedAnalysis.findings.map((finding) => (
                      <article className="rounded-lg border border-gray-200 p-3" key={`${finding.id}-${finding.code}`}>
                        <div className="flex items-center justify-between gap-3">
                          <p className="text-sm font-semibold text-gray-950">{localizedFinding(finding, locale).title}</p>
                          <span className="text-xs font-medium text-gray-500">{localizedEnum(finding.severity, locale)}</span>
                        </div>
                        <p className="mt-1 text-xs text-gray-500">{finding.code}</p>
                        <p className="mt-2 text-sm text-gray-600">{localizedFinding(finding, locale).description}</p>
                        {finding.explanation ? (
                          <p className="mt-3 text-sm text-gray-700">{localizedFinding(finding, locale).explanation}</p>
                        ) : null}
                        {finding.recommendation ? (
                          <p className="mt-2 text-sm font-medium text-gray-800">
                            {locale === "pt-BR" ? "Mitigação" : "Mitigation"}: {localizedFinding(finding, locale).recommendation}
                          </p>
                        ) : null}
                      </article>
                    ))}
                  </div>
                ) : (
                  <p className="mt-3 rounded-md bg-gray-50 px-4 py-3 text-sm text-gray-600">
                    {locale === "pt-BR" ? "Nenhum achado foi registrado para esta análise." : "No findings were recorded for this analysis."}
                  </p>
                )}
              </div>

              <div>
                <h3 className="text-sm font-semibold text-gray-950">{t("dashboard.metadata", "Review Metadata")}</h3>
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
                    {locale === "pt-BR" ? "Nenhum metadado foi registrado para esta análise." : "No metadata entries were recorded for this analysis."}
                  </p>
                )}
              </div>
            </div>

            {loadingAnalysis ? (
              <p className="mt-5 rounded-md bg-gray-50 px-4 py-3 text-sm text-gray-600">
                {locale === "pt-BR" ? "Carregando detalhes da análise." : "Loading analysis details."}
              </p>
            ) : null}

            {selectedAnalysis.inputType === "URL" && selectedAnalysis.file ? (
              <div className="mt-5 rounded-lg border border-amber-200 bg-amber-50/40 p-4">
                <h3 className="text-sm font-semibold text-gray-950">{locale === "pt-BR" ? "Inspeção de arquivo remoto" : "Remote File Inspection"}</h3>
                <div className="mt-3 space-y-3 text-sm text-gray-700">
                  <dl className="grid gap-2 sm:grid-cols-3">
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Origem" : "Source"}</dt>
                      <dd className="mt-1 text-gray-900">{locale === "pt-BR" ? "Baixado da URL enviada" : "Downloaded from submitted URL"}</dd>
                    </div>
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">MIME type</dt>
                      <dd className="mt-1 break-words text-gray-900">{selectedAnalysis.file.mimeType}</dd>
                    </div>
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Tamanho do arquivo" : "File size"}</dt>
                      <dd className="mt-1 text-gray-900">{formatBytes(selectedAnalysis.file.sizeBytes)}</dd>
                    </div>
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Nível de risco" : "Risk level"}</dt>
                      <dd className="mt-1 text-gray-900">{localizedEnum(selectedAnalysis.riskScore.level, locale)}</dd>
                    </div>
                  </dl>
                  <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                    {locale === "pt-BR" ? "Este arquivo remoto foi obtido e inspecionado no ambiente do backend antes de qualquer download local. Isso reduz a exposição direta, mas não garante a detecção de malware." : "This remote file was fetched into the backend inspection environment and inspected before any local download. This reduces direct exposure, but it does not guarantee malware detection."}
                  </p>
                  {selectedAnalysis.riskScore.level === "HIGH" ? (
                    <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                      {locale === "pt-BR" ? "Alto risco: este arquivo remoto original ainda pode conter conteúdo inseguro. Revise os achados com atenção antes de baixá-lo localmente." : "High risk: this original remote file may still contain unsafe content. Review findings carefully before choosing to download it locally."}
                    </p>
                  ) : null}
                </div>
              </div>
            ) : null}

            <div className={`mt-5 rounded-lg border p-4 ${selectedAnalysis.riskScore.level === "HIGH" ? "border-red-200 bg-red-50/30" : "border-amber-200 bg-amber-50/30"}`}>
              <h3 className="text-sm font-semibold text-gray-950">
                {originalFileHeading(selectedAnalysis, locale)}
              </h3>
              <p className="mt-1 text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Original potencialmente inseguro" : "Potentially unsafe original"}</p>
              {selectedAnalysis.file ? (
                <div className="mt-3 space-y-3 text-sm text-gray-700">
                  <dl className="grid gap-2 sm:grid-cols-3">
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Origem" : "Source"}</dt>
                      <dd className="mt-1 text-gray-900">{originalFileSource(selectedAnalysis, locale)}</dd>
                    </div>
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Nome do arquivo" : "Filename"}</dt>
                      <dd className="mt-1 break-words text-gray-900">{selectedAnalysis.file.originalFilename}</dd>
                    </div>
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">MIME type</dt>
                      <dd className="mt-1 break-words text-gray-900">{selectedAnalysis.file.mimeType}</dd>
                    </div>
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Tamanho" : "Size"}</dt>
                      <dd className="mt-1 text-gray-900">{formatBytes(selectedAnalysis.file.sizeBytes)}</dd>
                    </div>
                  </dl>
                  <p className={`rounded-md border px-3 py-2 text-sm ${selectedAnalysis.riskScore.level === "HIGH" ? "border-red-200 bg-red-50 text-red-700" : "border-amber-200 bg-amber-50 text-amber-800"}`}>
                    {originalFileWarning(selectedAnalysis, locale)}
                  </p>
                  {originalFileError ? (
                    <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                      {originalFileError}
                    </p>
                  ) : null}
                  <button
                    className={`rounded-md px-4 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-60 ${
                      selectedAnalysis.riskScore.level === "HIGH"
                        ? "border border-red-700 bg-red-700 text-white hover:bg-red-800"
                        : "border border-gray-300 bg-white text-gray-800 hover:bg-gray-50"
                    }`}
                    disabled={downloadingOriginalFile || !selectedAnalysis.file}
                    onClick={downloadOriginalFile}
                    type="button"
                  >
                    {downloadingOriginalFile ? t("common.loading", "Downloading...") : t("dashboard.downloadOriginal", "Download Original")}
                  </button>
                </div>
              ) : (
                <p className="mt-2 rounded-md bg-gray-50 px-4 py-3 text-sm text-gray-600">
                  {locale === "pt-BR" ? "Nenhum arquivo original está disponível para esta análise. Análises somente de URL não armazenam um arquivo para download, salvo quando a resposta é de um tipo compatível." : "No original file download is available for this analysis. URL-only analyses do not store a downloadable file unless the response is a supported file type."}
                </p>
              )}
            </div>

            <div className="mt-5 rounded-lg border border-gray-200 p-4">
              <h3 className="text-sm font-semibold text-gray-950">{t("dashboard.safePreview", "Review Preview")}</h3>
              <p className="mt-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                {t("dashboard.securityPreview", "This preview is static and passive. Active file content, website JavaScript, and browser behavior are not executed.")}
              </p>
              {selectedAnalysis.inputType === "URL" ? (
                <p className="mt-3 rounded-md border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700">
                  {locale === "pt-BR" ? "O conteúdo remoto foi inspecionado no ambiente do backend antes de qualquer download local, reduzindo a exposição direta e mantendo com você a decisão final de download." : "Remote content was inspected in the backend environment before any local download, reducing direct exposure while keeping the final download decision with you."}
                </p>
              ) : null}
              {selectedAnalysis.riskScore.level === "HIGH" ? (
                <p className="mt-3 rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                  {locale === "pt-BR" ? "Este conteúdo pode ser inseguro. Não confie nem abra localmente sem cautela." : "This content may be unsafe. Do not trust or open locally without caution."}
                </p>
              ) : null}
              {selectedAnalysis.safePreview?.available ? (
                <div className="mt-3">
                  {selectedAnalysis.safePreview.kind === "image" && selectedAnalysis.safePreview.dataUrl && !previewImageFailed ? (
                    <img
                      alt={locale === "pt-BR" ? "Pré-visualização segura estática" : "Static safe preview"}
                      className="max-h-[520px] max-w-full rounded-md border border-gray-200 bg-gray-50 object-contain"
                      onError={() => setPreviewImageFailed(true)}
                      src={selectedAnalysis.safePreview.dataUrl}
                    />
                  ) : null}
                  {selectedAnalysis.safePreview.kind === "text" && selectedAnalysis.safePreview.text ? (
                    <pre className="max-h-80 overflow-auto rounded-md border border-gray-200 bg-gray-50 p-3 text-sm text-gray-800 whitespace-pre-wrap">
                      {selectedAnalysis.safePreview.text}
                    </pre>
                  ) : null}
                  {previewShouldShowFallback(selectedAnalysis, previewImageFailed) ? (
                    <SafePreviewFallback locale={locale} message={safePreviewFallbackMessage(selectedAnalysis, previewImageFailed, locale)} />
                  ) : null}
                </div>
              ) : (
                <SafePreviewFallback locale={locale} message={safePreviewFallbackMessage(selectedAnalysis, previewImageFailed, locale)} />
              )}
            </div>

            <div className="mt-5 rounded-lg border border-sky-200 bg-sky-50/30 p-4">
              <h3 className="text-sm font-semibold text-gray-950">{t("dashboard.sanitizedFile", "Sanitized File")}</h3>
              <p className="mt-1 text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Somente cópia com metadados removidos" : "Metadata-cleaned copy only"}</p>
              {selectedAnalysis.cleanFile ? (
                <div className="mt-3 space-y-3 text-sm text-gray-700">
                  <dl className="grid gap-2 sm:grid-cols-3">
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Nome do arquivo" : "Filename"}</dt>
                      <dd className="mt-1 break-words text-gray-900">{selectedAnalysis.cleanFile.filename}</dd>
                    </div>
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Tamanho" : "Size"}</dt>
                      <dd className="mt-1 text-gray-900">{formatBytes(selectedAnalysis.cleanFile.sizeBytes)}</dd>
                    </div>
                    <div>
                      <dt className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Status da limpeza" : "Cleaning status"}</dt>
                      <dd className="mt-1 text-gray-900">{localizedEnum(selectedAnalysis.cleanFile.cleaningStatus, locale)}</dd>
                    </div>
                  </dl>
                  <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800">
                    {t("dashboard.sanitizedWarning", "This is a separate metadata-cleaned copy. It may still contain unsafe content and does not guarantee malware removal or full safety.")}
                  </p>
                  <div>
                    <p className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Metadados removidos" : "Metadata removed"}</p>
                    {selectedAnalysis.cleanFile.removedMetadataKeys.length > 0 ? (
                      <ul className="mt-1 list-disc space-y-1 pl-5 text-sm text-gray-700">
                        {describeRemovedMetadata(selectedAnalysis.cleanFile.removedMetadataKeys, locale).map((item) => (
                          <li key={item}>{item}</li>
                        ))}
                      </ul>
                    ) : (
                      <p className="mt-1 text-sm text-gray-600">{locale === "pt-BR" ? "Nenhum metadado removível compatível foi encontrado neste arquivo." : "No supported removable metadata was found in this file."}</p>
                    )}
                  </div>
                  <div>
                    <p className="text-xs font-semibold uppercase text-gray-500">{locale === "pt-BR" ? "Não removido" : "Not removed"}</p>
                    <ul className="mt-1 list-disc space-y-1 pl-5 text-sm text-gray-700">
                      <li>{locale === "pt-BR" ? "Malware, scripts, conteúdo incorporado e comportamento do documento não são removidos." : "Malware, scripts, embedded content, and document behavior are not removed."}</li>
                      <li>{locale === "pt-BR" ? "Metadados fora das regras atuais de limpeza de PDF/JPEG podem permanecer." : "Metadata outside the current PDF/JPEG cleanup rules may remain."}</li>
                    </ul>
                  </div>
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
                    {downloadingCleanFile ? t("common.loading", "Downloading...") : t("dashboard.downloadSanitized", "Download Sanitized Copy")}
                  </button>
                </div>
              ) : (
                <p className="mt-2 text-sm text-gray-600">
                  {locale === "pt-BR" ? "Nenhuma cópia sanitizada está disponível. O formato pode não ser compatível ou nenhuma saída com metadados removidos foi gerada." : "No sanitized copy is available. This format may be unsupported, or no metadata-cleaned output was generated."}
                </p>
              )}
            </div>
          </section>
        ) : null}
      </div>
    </main>
  );
}

function downloadGeneratedFile(filename: string, mimeType: string, content: string) {
  const url = URL.createObjectURL(new Blob([content], { type: mimeType }));
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

function buildStaticPDF(analysis: AnalysisDetail, locale: "en" | "pt-BR") {
  const lines = [
    locale === "pt-BR" ? "Relatorio estatico de analise DataGuardian" : "DataGuardian static analysis report",
    `${locale === "pt-BR" ? "Analise" : "Analysis"}: ${analysis.analysisId}`,
    `${locale === "pt-BR" ? "Entrada" : "Input"}: ${analysis.inputType}`,
    `Status: ${analysis.status}`,
    `${locale === "pt-BR" ? "Risco" : "Risk"}: ${analysis.riskScore.level} (${analysis.riskScore.score})`,
    `${locale === "pt-BR" ? "Resumo" : "Summary"}: ${analysis.summary}`,
    ...analysis.findings.map((finding) => `${finding.severity} ${finding.code}: ${finding.title}`),
    locale === "pt-BR" ? "Copias sanitizadas removem apenas metadados suportados e ainda podem conter conteudo malicioso." : "Sanitized copies remove supported metadata only and may still contain malicious content.",
  ].map((line) => line.replace(/[^\x20-\x7E]/g, "?").slice(0, 100));
  const stream = `BT /F1 11 Tf 50 790 Td ${lines.map((line, index) => `${index ? "0 -18 Td " : ""}(${line.replace(/[\\()]/g, "\\$&")}) Tj`).join(" ")} ET`;
  const objects = [
    "1 0 obj << /Type /Catalog /Pages 2 0 R >> endobj",
    "2 0 obj << /Type /Pages /Kids [3 0 R] /Count 1 >> endobj",
    "3 0 obj << /Type /Page /Parent 2 0 R /MediaBox [0 0 612 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >> endobj",
    "4 0 obj << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> endobj",
    `5 0 obj << /Length ${stream.length} >> stream\n${stream}\nendstream endobj`,
  ];
  let pdf = "%PDF-1.4\n";
  const offsets = [0];
  for (const object of objects) { offsets.push(pdf.length); pdf += `${object}\n`; }
  const xref = pdf.length;
  pdf += `xref\n0 6\n0000000000 65535 f \n${offsets.slice(1).map((offset) => `${String(offset).padStart(10, "0")} 00000 n `).join("\n")}\n`;
  return `${pdf}trailer << /Size 6 /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF`;
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

function localizedEnum(value: string, locale: "en" | "pt-BR") {
  if (locale !== "pt-BR") return value.charAt(0) + value.slice(1).toLowerCase();
  return ({ LOW: "Baixo", MEDIUM: "Médio", HIGH: "Alto", COMPLETED: "Concluído", PROCESSING: "Processando", FAILED: "Falhou", PENDING: "Pendente", FILE: "Arquivo", URL: "URL", INFO: "Informativo", WARNING: "Alerta", CRITICAL: "Crítico" } as Record<string, string>)[value] ?? value;
}

function localizedSummary(summary: string, locale: "en" | "pt-BR") {
  if (locale !== "pt-BR") return summary;
  const summaries: Record<string, string> = {
    "File analysis completed with no findings.": "Análise do arquivo concluída sem achados.",
    "File analysis completed with structured findings.": "Análise do arquivo concluída com achados estruturados.",
    "URL analysis completed with no findings.": "Análise da URL concluída sem achados.",
    "URL analysis completed with structured findings.": "Análise da URL concluída com achados estruturados.",
    "URL analysis completed with remote file inspection.": "Análise da URL concluída com inspeção do arquivo remoto.",
    "URL analysis completed with a remote file inspection candidate.": "Análise da URL concluída com um candidato a inspeção de arquivo remoto.",
  };
  return summaries[summary] ?? summary;
}

const ptBRFindingTitles: Record<string, string> = {
  PDF_JS_DETECTED: "Marcador de JavaScript em PDF detectado",
  PDF_OPENACTION_DETECTED: "Ação automática de abertura em PDF detectada",
  GENERIC_BASE64_PATTERN: "Padrão Base64 detectado",
  GENERIC_EVAL_PATTERN: "Padrão de execução dinâmica detectado",
  METADATA_GPS_EXPOSED: "Metadados de localização expostos",
  METADATA_AUTHOR_PRESENT: "Metadados de autoria presentes",
  METADATA_SUSPICIOUS_PRESENT: "Metadados suspeitos presentes",
  URL_NO_HTTPS: "A URL não utiliza HTTPS",
  URL_REDIRECT_DETECTED: "Redirecionamento de URL detectado",
  URL_FETCH_FAILED: "Falha ao obter a URL",
  URL_SUSPICIOUS_CONTENT: "Conteúdo suspeito detectado na URL",
  URL_REMOTE_FILE_DETECTED: "Arquivo remoto detectado",
};

function localizedFinding(finding: AnalysisFinding, locale: "en" | "pt-BR") {
  if (locale !== "pt-BR") return finding;
  return {
    ...finding,
    title: ptBRFindingTitles[finding.code] ?? finding.title,
    description: `O analisador passivo identificou o indicador ${finding.code}. Consulte o código do achado e trate o conteúdo como não confiável.`,
    explanation: finding.explanation ? "Este indicador pode aumentar o risco do conteúdo. A classificação é determinística e não implica execução nem confirmação de malware." : "",
    recommendation: finding.recommendation ? "Revise o achado, faça uma verificação independente e evite abrir ou executar o conteúdo em um ambiente confiável." : null,
  };
}

function formatDate(value: string, locale: "en" | "pt-BR") {
  return new Intl.DateTimeFormat(locale, {
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

function originalFileHeading(analysis: AnalysisDetail, locale: "en" | "pt-BR") {
  if (locale === "pt-BR") return analysis.inputType === "URL" ? "Arquivo remoto original" : "Arquivo enviado original";
  return analysis.inputType === "URL" ? "Original Remote File" : "Original Uploaded File";
}

function originalFileSource(analysis: AnalysisDetail, locale: "en" | "pt-BR") {
  if (locale === "pt-BR") return analysis.inputType === "URL" ? "Resposta da URL remota" : "Envio local";
  return analysis.inputType === "URL" ? "Remote URL response" : "Local upload";
}

function originalFileWarning(analysis: AnalysisDetail, locale: "en" | "pt-BR") {
  const source = analysis.inputType === "URL" ? "remote file" : "uploaded file";
  if (locale === "pt-BR") {
    const origem = analysis.inputType === "URL" ? "arquivo remoto" : "arquivo enviado";
    if (analysis.riskScore.level === "HIGH") return `Alto risco: este ${origem} original é preservado sem alterações e ainda pode conter conteúdo inseguro. O DataGuardian reduz a exposição com inspeção passiva, mas não garante a detecção de malware.`;
    return `Este ${origem} original é preservado sem alterações. Revise a prévia, os achados e a pontuação de risco antes de baixar ou abrir localmente.`;
  }
  if (analysis.riskScore.level === "HIGH") {
    return `High risk: this original ${source} is preserved unchanged and may still contain unsafe content. DataGuardian reduces exposure with passive inspection, but it does not guarantee malware detection.`;
  }
  if (analysis.inputType === "URL") {
    return "This original remote file was inspected before local download and is preserved unchanged. Review the preview, findings, and risk score before downloading or opening it locally.";
  }
  return `This original ${source} is preserved unchanged. Review the preview, findings, and risk score before downloading or opening it locally.`;
}

function previewShouldShowFallback(analysis: AnalysisDetail, imageFailed: boolean) {
  const preview = analysis.safePreview;
  if (!preview?.available) {
    return true;
  }
  if (preview.kind === "image") {
    return imageFailed || !preview.dataUrl;
  }
  if (preview.kind === "text") {
    return !preview.text?.trim();
  }
  return true;
}

function safePreviewFallbackMessage(analysis: AnalysisDetail, imageFailed: boolean, locale: "en" | "pt-BR") {
  if (imageFailed) {
    if (locale === "pt-BR") return "Não foi possível exibir a pré-visualização segura. O arquivo ainda foi inspecionado passivamente; revise os achados e metadados antes de decidir pelo download.";
    return "Safe preview could not be displayed. The file was still inspected passively; review findings and metadata before deciding whether to download.";
  }
  if (locale === "pt-BR") return "Pré-visualização indisponível para este conteúdo. Revise os achados, metadados e risco antes de decidir pelo download.";
  return analysis.safePreview?.message ?? "Preview unavailable for this content. Review findings, metadata, and risk before deciding whether to download.";
}

function SafePreviewFallback({ message, locale }: { message: string; locale: "en" | "pt-BR" }) {
  return (
    <div className="mt-3 rounded-md border border-gray-200 bg-gray-50 px-4 py-5 text-sm text-gray-700">
      <p className="font-semibold text-gray-900">{locale === "pt-BR" ? "Pré-visualização indisponível" : "Preview unavailable"}</p>
      <p className="mt-1">{message}</p>
      <p className="mt-2 text-xs text-gray-500">
        {locale === "pt-BR" ? "O DataGuardian não executa conteúdo ativo para gerar pré-visualizações." : "DataGuardian does not execute active content to generate previews."}
      </p>
    </div>
  );
}

function confirmHighRiskOriginalDownload(analysis: AnalysisDetail) {
  const source = analysis.inputType === "URL" ? "remote file" : "uploaded file";
  const inspectedNote =
    analysis.inputType === "URL"
      ? "It was inspected in the backend before local download, reducing direct exposure, but malware detection is not guaranteed."
      : "It was inspected passively, but malware detection is not guaranteed.";
  const expected = "DOWNLOAD ORIGINAL";
  const response = window.prompt(
    `High-risk original ${source}. ${inspectedNote} Type ${expected} to download the unchanged original file.`,
  );
  return response === expected;
}

function describeRemovedMetadata(keys: string[], locale: "en" | "pt-BR") {
  const descriptions = new Set<string>();
  keys.forEach((key) => {
    switch (key) {
      case "exif":
        descriptions.add(locale === "pt-BR" ? "Metadados EXIF removidos, incluindo campos de GPS/localização quando presentes." : "EXIF metadata removed, including GPS/location fields when present.");
        break;
      case "gps":
        descriptions.add(locale === "pt-BR" ? "Metadados de GPS/localização removidos." : "GPS/location metadata removed.");
        break;
      case "author":
        descriptions.add(locale === "pt-BR" ? "Metadados de autoria removidos." : "Author metadata removed.");
        break;
      case "producer":
        descriptions.add(locale === "pt-BR" ? "Metadados de produtor/ferramenta removidos." : "Producer/tool metadata removed.");
        break;
      case "creation_date":
      case "timestamp":
      case "datetime":
        descriptions.add(locale === "pt-BR" ? "Metadados de data e hora removidos." : "Timestamp metadata removed.");
        break;
      default:
        descriptions.add(locale === "pt-BR" ? `${key.replaceAll("_", " ")} removido.` : `${key.replaceAll("_", " ")} removed.`);
    }
  });
  return Array.from(descriptions);
}
