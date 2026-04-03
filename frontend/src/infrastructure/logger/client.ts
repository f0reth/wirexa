import { Log } from "../../../wailsjs/go/adapters/LogHandler";

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
