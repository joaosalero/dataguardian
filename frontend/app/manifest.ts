import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "DataGuardian",
    short_name: "DataGuardian",
    description: "Local passive inspection for suspicious files and URLs",
    start_url: "/dashboard",
    display: "standalone",
    background_color: "#f6f7f9",
    theme_color: "#111827",
  };
}
