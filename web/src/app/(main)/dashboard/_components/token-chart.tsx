"use client";

import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";

import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";

export interface TokenChartPoint {
  date: string;
  input: number;
  output: number;
  cacheRead: number;
}

export type TokenChartSeries = Record<string, { label: string; color: string }>;

function fmtTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return String(n);
}

// Loaded on demand by the dashboard so Recharts stays out of the first paint.
export function TokenChart({
  data,
  config,
  emptyLabel,
}: {
  data: TokenChartPoint[];
  config: TokenChartSeries;
  emptyLabel: string;
}) {
  if (data.length === 0) {
    return (
      <div
        className="flex flex-1 items-center justify-center rounded-lg border bg-muted/10 text-muted-foreground text-xs"
        style={{ minHeight: 180 }}
      >
        {emptyLabel}
      </div>
    );
  }

  return (
    <ChartContainer config={config} className="h-[200px] w-full">
      <BarChart data={data} margin={{ top: 4, right: 4, bottom: 0, left: 0 }} maxBarSize={40}>
        <CartesianGrid vertical={false} />
        <XAxis
          dataKey="date"
          tickLine={false}
          axisLine={false}
          tick={{ fontSize: 10 }}
          tickMargin={6}
          interval="preserveStartEnd"
        />
        <YAxis
          tickFormatter={(v) => fmtTokens(Number(v))}
          tickLine={false}
          axisLine={false}
          tick={{ fontSize: 10 }}
          width={44}
        />
        <ChartTooltip
          cursor={false}
          content={
            <ChartTooltipContent
              indicator="line"
              formatter={(value, name) => (
                <div className="flex w-full items-center justify-between gap-4">
                  <span>{config[String(name)]?.label ?? String(name)}</span>
                  <span className="font-mono font-semibold tabular-nums">{fmtTokens(Number(value))}</span>
                </div>
              )}
            />
          }
        />
        <Bar dataKey="input" stackId="1" fill="var(--color-input)" radius={[0, 0, 0, 0]} />
        <Bar dataKey="output" stackId="1" fill="var(--color-output)" radius={[0, 0, 0, 0]} />
        <Bar dataKey="cacheRead" stackId="1" fill="var(--color-cacheRead)" radius={[4, 4, 0, 0]} />
      </BarChart>
    </ChartContainer>
  );
}
