"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useI18n } from "../../i18n";

export default function ProjectPage() {
  const router = useRouter();
  const { locale } = useI18n();

  useEffect(() => {
    router.replace("/dashboard");
  }, [router]);

  return (
    <main className="min-h-screen px-6 py-8">
      <div className="mx-auto max-w-3xl">
        <p className="text-sm text-gray-600">{locale === "pt-BR" ? "Redirecionando ao painel..." : "Redirecting to dashboard..."}</p>
      </div>
    </main>
  );
}
