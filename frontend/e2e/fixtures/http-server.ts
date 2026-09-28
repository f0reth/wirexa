import * as http from "node:http";
import type { AddressInfo } from "node:net";

export interface TestServer {
  port: number;
  close(): Promise<void>;
}

/**
 * 受け取ったリクエストをワイヤに近い形のテキストで返すハンドラ。
 * リクエスト行 (メソッドとクエリ付きのパス)、受信した順のヘッダー (名前は小文字)、空行、
 * ボディをそのまま並べる。ボディは解析しないので、multipart の組み立ても直接確かめられる。
 */
export function echoRequest(
  req: http.IncomingMessage,
  res: http.ServerResponse,
): void {
  const chunks: Buffer[] = [];
  req.on("data", (c: Buffer) => chunks.push(c));
  req.on("end", () => {
    const lines = [`${req.method} ${req.url}`];
    for (let i = 0; i < req.rawHeaders.length; i += 2) {
      lines.push(`${req.rawHeaders[i].toLowerCase()}: ${req.rawHeaders[i + 1]}`);
    }
    res.writeHead(200, { "Content-Type": "text/plain" });
    res.end(
      Buffer.concat([Buffer.from(`${lines.join("\n")}\n\n`), ...chunks]),
    );
  });
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
