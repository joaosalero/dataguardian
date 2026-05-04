"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function ProjectPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace("/dashboard");
  }, [router]);

  return (
    <main className="min-h-screen px-6 py-8">
      <div className="mx-auto max-w-3xl">
        <p className="text-sm text-gray-600">Redirecting to dashboard...</p>
      </div>
    </main>
  );
}
