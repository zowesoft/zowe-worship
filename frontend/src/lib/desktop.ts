export function hasDesktopBridge(): boolean {
  return Boolean((window as Window & { go?: unknown }).go);
}
