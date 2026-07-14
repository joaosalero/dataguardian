import { redirect } from "next/navigation";

export default async function AnalysisLinkPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  if (!/^\d+$/.test(id)) redirect("/dashboard");
  redirect(`/dashboard?analysis=${encodeURIComponent(id)}`);
}
