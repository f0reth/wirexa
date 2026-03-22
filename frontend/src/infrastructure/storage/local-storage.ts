export function loadFromStorage<T>(key: string, fallback: T): T {
  const item = localStorage.getItem(key);
  if (item === null) return fallback;
  try {
    return JSON.parse(item) as T;
  } catch (error) {
    console.warn(
      `[storage] corrupt data for key "${key}", using fallback:`,
      error,
    );
    return fallback;
  }
}

export function saveToStorage<T>(key: string, value: T): boolean {
  try {
    localStorage.setItem(key, JSON.stringify(value));
    return true;
  } catch (error) {
    console.warn(`[storage] saveToStorage failed for key "${key}":`, error);
    return false;
  }
}

export function removeFromStorage(key: string): void {
  try {
    localStorage.removeItem(key);
  } catch (error) {
    console.warn(`[storage] removeFromStorage failed for key "${key}":`, error);
  }
}
