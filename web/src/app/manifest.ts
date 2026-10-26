import type { MetadataRoute } from "next";

import { APP_CONFIG } from "@/config/app-config";

// Static export has no server runtime, so the manifest must be prerendered.
export const dynamic = "force-static";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: APP_CONFIG.name,
    short_name: APP_CONFIG.name,
    description: APP_CONFIG.meta.description,
    start_url: "/",
    display: "standalone",
    background_color: "#0B0A12",
    theme_color: "#7C3AED",
    icons: [
      { src: "/brand/hartex-icon-192.png", sizes: "192x192", type: "image/png" },
      { src: "/brand/hartex-icon-512.png", sizes: "512x512", type: "image/png" },
    ],
  };
}
