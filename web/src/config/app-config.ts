const currentYear = new Date().getFullYear();

export const APP_CONFIG = {
  name: "Hartex",
  wordmark: "HARTEX",
  copyright: `© ${currentYear}, Hartex.`,
  // Browser tab title and search-engine meta description, rendered into every page's <title>
  // and <meta name="description"> via layout.tsx's static metadata.
  meta: {
    title: "Hartex: Autonomous Penetration Testing Console",
    description: "An LLM-driven console for autonomous penetration testing",
  },
};
