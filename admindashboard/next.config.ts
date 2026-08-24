import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/healthcheck",
        // Rewrites to a built-in static asset path that always returns 200 OK
        destination: "/healthcheck", // Points directly to app router endpoint
      },
    ];
  },
};

export default nextConfig;
