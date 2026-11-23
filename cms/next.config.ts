import type { NextConfig } from "next";

const config: NextConfig = {
  reactStrictMode: true,
  // keep the dev badge off the navigation rail
  devIndicators: { position: "bottom-right" },
};

export default config;
