export function loadFromStorage<T>(key: string, fallback: T): T {
  try {
    const item = localStorage.getItem(key);
    if (item === null) return fallback;
    return JSON.parse(item) as T;
  } catch {
    return fallback;
  }
}

export function saveToStorage<T>(key: string, value: T): void {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // quota exceeded など — 握りつぶす
  }
}

export function removeFromStorage(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch {
    // ignore
  }
}
