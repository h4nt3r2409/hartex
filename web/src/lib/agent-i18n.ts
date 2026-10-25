import { useCallback, useEffect, useMemo, useState } from "react";

import { useTranslations } from "next-intl";

import { api } from "@/lib/api";
import type { Agent } from "@/lib/types";

// Built-in agent names, descriptions, and prompt-variable labels are DB seed data
// (builtinAgents in db/db.go, the vulnerability-retest agent in server/finding_retests.go).
// The seeds in this tree are English, but the seed uses ON CONFLICT DO NOTHING, so it is not
// applied to databases installed by an older upstream build. The UI therefore looks up labels
// by key in the builtinAgents namespace in web/messages/en.json (which keeps those legacy DB
// names masked) and shows them when present, otherwise uses the DB value as-is. User-created
// agents always use the DB value.

// retester is seeded into the DB with builtin=false (server FindingRetestAgentKey), but it
// is a fixed key planted by the system, so it is included in translation. Agents with
// builtin=true (goals/planner/mainagent/worker/auto/pentest) are filtered by agent.builtin,
// so they do not need to be listed here.
const SYSTEM_SEEDED_AGENT_KEYS = new Set(["retester"]);

function isSystemSeeded(agent: { key: string; builtin?: boolean }): boolean {
  return !!agent.builtin || SYSTEM_SEEDED_AGENT_KEYS.has(agent.key);
}

// useAgentLabels is a bundle of helpers that returns built-in agent names, descriptions,
// and prompt-variable labels in the current language. Every display site obtains labels
// through these helpers.
export function useAgentLabels() {
  const t = useTranslations("builtinAgents");

  // Callers use these functions as useMemo/useCallback dependencies (conversation grouping,
  // filter computation), so build a new bundle only when the language (t) changes. A new
  // object on every render would rerun those computations each time.
  return useMemo(() => {
    // name returns the translated name for a system-seeded agent, otherwise the DB name (agent.name).
    const name = (agent: { key: string; name: string; builtin?: boolean }): string => {
      if (isSystemSeeded(agent) && t.has(`${agent.key}.name`)) return t(`${agent.key}.name`);
      return agent.name;
    };

    // description returns the translated description for a system-seeded agent, otherwise
    // the DB description (or an empty string). The caller still appends its existing fallback
    // text for empty descriptions.
    const description = (agent: { key: string; description?: string; builtin?: boolean }): string => {
      if (isSystemSeeded(agent) && t.has(`${agent.key}.description`)) return t(`${agent.key}.description`);
      return agent.description ?? "";
    };

    // nameByKey is the variant used when only a key is available without an Agent object
    // (e.g. grouping conversations). It takes a lookup function returning the agent and uses
    // the translation when found, otherwise returns the key.
    const nameByKey = (
      key: string,
      lookup: (key: string) => { key: string; name: string; builtin?: boolean } | undefined,
    ): string => {
      const agent = lookup(key);
      // biome-ignore lint/nursery/useNullishCoalescing: agents with an empty ("") name must still fall back to the key as before, so || is intentional.
      return (agent && name(agent)) || key;
    };

    // varLabel translates a built-in agent's prompt-variable label (e.g. "Overall task
    // goal" for goals). When no translation exists, the DB label (fallback) is used as-is.
    const varLabel = (agentKey: string, builtin: boolean, varName: string, fallback: string): string => {
      if ((builtin || SYSTEM_SEEDED_AGENT_KEYS.has(agentKey)) && t.has(`${agentKey}.vars.${varName}`)) {
        return t(`${agentKey}.vars.${varName}`);
      }
      return fallback;
    };

    return { name, description, nameByKey, varLabel };
  }, [t]);
}

// Several screens (approval history, etc.) use the agent list only to map key → display
// name, so fetch it once and share it at module level. On failure, the next call retries.
let agentListPromise: Promise<Agent[]> | null = null;
function loadAgentList(): Promise<Agent[]> {
  if (!agentListPromise) {
    agentListPromise = api.agents().catch((err) => {
      agentListPromise = null;
      throw err;
    });
  }
  return agentListPromise;
}

// useAgentNameByKey returns a converter for screens that have only a key, no Agent object
// (e.g. conv_agent_key in approval/intercept records). Built-in agents use the current-
// language name, user agents use the DB name, and keys missing from the list or not yet
// loaded are shown as-is.
export function useAgentNameByKey(): (key: string) => string {
  const labels = useAgentLabels();
  const [agents, setAgents] = useState<Map<string, Agent>>(() => new Map());
  useEffect(() => {
    let alive = true;
    loadAgentList()
      .then((list) => {
        if (alive) setAgents(new Map(list.map((a) => [a.key, a])));
      })
      .catch(() => {
        // If the list cannot be fetched, show the key as-is (retry on the next mount).
      });
    return () => {
      alive = false;
    };
  }, []);
  return useCallback((key: string) => labels.nameByKey(key, (k) => agents.get(k)), [labels, agents]);
}
