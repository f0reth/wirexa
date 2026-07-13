import * as http from "node:http";
import type { AddressInfo } from "node:net";

export interface TestServer {
  port: number;
  close(): Promise<void>;
}

/**
 * テスト用の HTTP サーバーを起動し、実際に割り当てられたポートを返す。
 *
 * ポート 0 で listen したまま addr を読む。「空きポートを探して閉じ、番号だけ返して後で bind し直す」
 * よくあるヘルパーは、その隙間に他プロセスがポートを奪える (TOCTOU) ためフレークの種になる。
 */
export async function startTestServer(
  handler: http.RequestListener,
): Promise<TestServer> {
  const server = http.createServer(handler);
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const port = (server.address() as AddressInfo).port;

  return {
    port,
    close: () => new Promise<void>((resolve) => server.close(() => resolve())),
  };
}
