import "@testing-library/jest-dom/vitest";

if (typeof window !== "undefined") {
  if (typeof window.HTMLElement !== "undefined" && !window.HTMLElement.prototype.scrollIntoView) {
    window.HTMLElement.prototype.scrollIntoView = () => {};
  }

  if (
    !window.localStorage ||
    typeof window.localStorage.getItem !== "function" ||
    typeof window.localStorage.setItem !== "function"
  ) {
    const store = new Map<string, string>();
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      value: {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => {
          store.set(key, value);
        },
        removeItem: (key: string) => {
          store.delete(key);
        },
        clear: () => {
          store.clear();
        }
      }
    });
  }
}
