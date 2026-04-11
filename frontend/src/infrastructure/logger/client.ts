import { Log } from "../../../wailsjs/go/adapters/LogHandler";
import type { Logger } from "../../application/logger";

export type LogLevel = "INFO" | "ERROR" | "DEBUG";

export interface LogEntry {
  level: LogLevel;
  source: string;
  message: string;
  attrs?: Record<string, unknown>;
}

// log はフロントエンドのログエントリを Go 側のファイルロガーへ送る（fire-and-forget）。
export function log(entry: LogEntry): void {
  void Log(entry as never);
}

export function createLogger(source: string): Logger {
  return {
    info: (msg, attrs) => log({ level: "INFO", source, message: msg, attrs }),
    error: (msg, attrs) => log({ level: "ERROR", source, message: msg, attrs }),
  };
}
