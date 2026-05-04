"use client";

import Link from "next/link";
import { FormEvent, useEffect, useState } from "react";
import { useRouter } from "next/navigation";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8000";

type Project = {
  id: number;
  name: string;
  description: string | null;
  created_at: string;
};

export default function DashboardPage() {
  const router = useRouter();
  const [projects, setProjects] = useState<Project[]>([]);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);

  async function loadProjects() {
    setError("");
    const response = await fetch(`${API_BASE_URL}/projects`, {
      credentials: "include",
    });

    if (response.status === 401) {
      router.push("/login");
      return;
    }

    if (!response.ok) {
      throw new Error("Something went wrong while loading projects.");
    }

    setProjects(await response.json());
  }

  useEffect(() => {
    loadProjects()
      .catch((err) =>
        setError(err instanceof Error ? err.message : "Something went wrong."),
      )
      .finally(() => setLoading(false));
  }, []);

  async function createProject(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();

    setError("");
    setMessage("");
    setCreating(true);
    try {
      const response = await fetch(`${API_BASE_URL}/projects`, {
        method: "POST",
        credentials: "include",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          name,
          description: description || null,
        }),
      });

      if (response.status === 401) {
        router.push("/login");
        return;
      }

      if (!response.ok) {
        setError("Could not create project. Check the name and try again.");
        return;
      }

      setName("");
      setDescription("");
      setMessage("Project created.");
      await loadProjects();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong.");
    } finally {
      setCreating(false);
    }
  }

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
            <h1 className="text-3xl font-semibold text-gray-950">Projects</h1>
            <p className="mt-1 text-sm text-gray-600">
              Run audits and review recent results.
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

        <section className="mb-6 rounded-lg border border-gray-200 bg-white p-5 shadow-sm">
          <h2 className="mb-4 text-base font-semibold text-gray-950">
            New project
          </h2>
          <form
            className="grid gap-3 md:grid-cols-[1fr_1fr_auto]"
            onSubmit={createProject}
          >
            <input
              className="rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
              disabled={creating}
              placeholder="Project name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              required
            />
            <input
              className="rounded-md border border-gray-300 px-3 py-2 text-sm outline-none focus:border-gray-900"
              disabled={creating}
              placeholder="Description"
              value={description}
              onChange={(event) => setDescription(event.target.value)}
            />
            <button
              className="rounded-md bg-gray-950 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 disabled:cursor-not-allowed disabled:bg-gray-400"
              disabled={creating || !name.trim()}
              type="submit"
            >
              {creating ? "Creating..." : "Create"}
            </button>
          </form>
        </section>

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

        <section className="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm">
          {loading ? (
            <p className="p-5 text-sm text-gray-600">Loading projects...</p>
          ) : projects.length === 0 ? (
            <p className="p-5 text-sm text-gray-600">No projects yet.</p>
          ) : (
            <div className="divide-y divide-gray-200">
              {projects.map((project) => (
                <Link
                  className="block p-5 hover:bg-gray-50"
                  href={`/projects/${project.id}`}
                  key={project.id}
                >
                  <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
                    <div>
                      <h2 className="font-medium text-gray-950">{project.name}</h2>
                      <p className="mt-1 text-sm text-gray-600">
                        {project.description ?? "No description"}
                      </p>
                    </div>
                    <span className="text-sm text-gray-500">
                      {new Date(project.created_at).toLocaleDateString()}
                    </span>
                  </div>
                </Link>
              ))}
            </div>
          )}
        </section>
      </div>
    </main>
  );
}
