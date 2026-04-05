import type { adapters } from "../models";

export function Log(_entry: adapters.LogEntry): Promise<void> {
  return Promise.resolve();
}
