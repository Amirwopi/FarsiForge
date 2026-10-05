import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'export',
  // Next.js static exports don't support Image Optimization API out of the box,
  // but we aren't using next/image, so it's fine.
};

export default nextConfig;
