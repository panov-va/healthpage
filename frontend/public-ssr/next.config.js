/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Автономная сборка для тонкого прод-образа (Docker): копируется только нужный runtime.
  output: "standalone",
};

module.exports = nextConfig;
