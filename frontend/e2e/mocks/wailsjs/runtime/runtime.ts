import { store } from "../store";

export function EventsOn(
  eventName: string,
  callback: (...data: unknown[]) => void,
): () => void {
  const handlers = store.eventHandlers.get(eventName) ?? [];
  handlers.push(callback);
  store.eventHandlers.set(eventName, handlers);
  return () => {
    EventsOff(eventName);
  };
}

export function EventsEmit(eventName: string, ...data: unknown[]): void {
  const handlers = store.eventHandlers.get(eventName) ?? [];
  for (const h of handlers) {
    h(...data);
  }
}

export function EventsOff(
  eventName: string,
  ...additionalEventNames: string[]
): void {
  store.eventHandlers.delete(eventName);
  for (const name of additionalEventNames) {
    store.eventHandlers.delete(name);
  }
}

export function EventsOffAll(): void {
  store.eventHandlers.clear();
}

export function EventsOnMultiple(
  eventName: string,
  callback: (...data: unknown[]) => void,
  _maxCallbacks: number,
): () => void {
  return EventsOn(eventName, callback);
}

export function EventsOnce(
  eventName: string,
  callback: (...data: unknown[]) => void,
): () => void {
  return EventsOn(eventName, callback);
}

export function LogPrint(message: string): void {
  console.log(message);
}

export function LogTrace(message: string): void {
  console.trace(message);
}

export function LogDebug(message: string): void {
  console.debug(message);
}

export function LogError(message: string): void {
  console.error(message);
}

export function LogFatal(message: string): void {
  console.error("FATAL:", message);
}

export function LogInfo(message: string): void {
  console.info(message);
}

export function LogWarning(message: string): void {
  console.warn(message);
}

export function WindowSetSystemDefaultTheme(): void {}
export function WindowSetLightTheme(): void {}
export function WindowSetDarkTheme(): void {}
export function BrowserOpenURL(_url: string): void {}
export function Quit(): void {}
