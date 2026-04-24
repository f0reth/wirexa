export function DeleteTarget(_arg1: string): Promise<void> {
  return Promise.resolve();
}
export function GetListeners(): Promise<unknown[]> {
  return Promise.resolve([]);
}
export function GetTargets(): Promise<unknown[]> {
  return Promise.resolve([]);
}
export function SaveTarget(arg1: unknown): Promise<unknown> {
  return Promise.resolve(arg1);
}
export function Send(_arg1: unknown): Promise<unknown> {
  return Promise.resolve({ bytesSent: 0 });
}
export function Shutdown(): Promise<void> {
  return Promise.resolve();
}
export function StartListen(_arg1: number, _arg2: string): Promise<unknown> {
  return Promise.resolve({ id: "", port: 0, encoding: "ascii" });
}
export function StopListen(_arg1: string): Promise<void> {
  return Promise.resolve();
}
