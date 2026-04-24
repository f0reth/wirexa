export function LogPrint(_message: string) {}
export function LogTrace(_message: string) {}
export function LogDebug(_message: string) {}
export function LogInfo(_message: string) {}
export function LogWarning(_message: string) {}
export function LogError(_message: string) {}
export function LogFatal(_message: string) {}

export function EventsOnMultiple(
  _eventName: string,
  _callback: (...data: unknown[]) => void,
  _maxCallbacks: number,
): () => void {
  return () => {};
}

export function EventsOn(
  _eventName: string,
  _callback: (...data: unknown[]) => void,
): () => void {
  return () => {};
}

export function EventsOff(_eventName: string, ..._rest: string[]): void {}
export function EventsOffAll(): void {}
export function EventsOnce(
  _eventName: string,
  _callback: (...data: unknown[]) => void,
): () => void {
  return () => {};
}
export function EventsEmit(_eventName: string, ..._data: unknown[]): void {}

export function WindowReload() {}
export function WindowReloadApp() {}
export function BrowserOpenURL(_url: string) {}
export function ClipboardGetText(): Promise<string> {
  return Promise.resolve("");
}
export function ClipboardSetText(_text: string): Promise<boolean> {
  return Promise.resolve(true);
}
