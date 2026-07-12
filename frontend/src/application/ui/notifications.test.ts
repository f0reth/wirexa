import { afterEach, describe, expect, it, vi } from "vitest";
import { createNotificationStore } from "./notifications";

afterEach(() => {
  vi.useRealTimers();
});

describe("createNotificationStore", () => {
  it("adds a notification via notify and exposes it", () => {
    const store = createNotificationStore();
    const id = store.notify.error("boom", "details");
    const list = store.notifications();
    expect(list).toHaveLength(1);
    expect(list[0]).toMatchObject({
      id,
      level: "error",
      title: "boom",
      description: "details",
    });
  });

  it("dismiss removes the matching notification only", () => {
    const store = createNotificationStore();
    const a = store.notify.info("a");
    store.notify.info("b");
    store.dismiss(a);
    const titles = store.notifications().map((n) => n.title);
    expect(titles).toEqual(["b"]);
  });

  it("clear removes all notifications", () => {
    const store = createNotificationStore();
    store.notify.success("x");
    store.notify.warning("y");
    store.clear();
    expect(store.notifications()).toHaveLength(0);
  });

  it("drops the oldest when exceeding max", () => {
    const store = createNotificationStore({ max: 2 });
    store.notify.info("1");
    store.notify.info("2");
    store.notify.info("3");
    const titles = store.notifications().map((n) => n.title);
    expect(titles).toEqual(["2", "3"]);
  });

  it("suppresses duplicates sharing the same key", () => {
    const store = createNotificationStore();
    const first = store.notify.error("lost", "conn-1", { key: "conn-1" });
    const second = store.notify.error("lost again", "conn-1", {
      key: "conn-1",
    });
    expect(second).toBe(first);
    expect(store.notifications()).toHaveLength(1);
    expect(store.notifications()[0].title).toBe("lost");
  });

  it("allows a new keyed notification after the previous one is dismissed", () => {
    const store = createNotificationStore();
    const first = store.notify.error("lost", undefined, { key: "conn-1" });
    store.dismiss(first);
    const second = store.notify.error("lost", undefined, { key: "conn-1" });
    expect(second).not.toBe(first);
    expect(store.notifications()).toHaveLength(1);
  });

  it("auto-dismisses non-error after autoDismissMs", () => {
    vi.useFakeTimers();
    const store = createNotificationStore({
      autoDismissMs: 1000,
      errorDismissMs: 5000,
    });
    store.notify.info("temp");
    expect(store.notifications()).toHaveLength(1);
    vi.advanceTimersByTime(1000);
    expect(store.notifications()).toHaveLength(0);
  });

  it("keeps errors longer than the non-error duration", () => {
    vi.useFakeTimers();
    const store = createNotificationStore({
      autoDismissMs: 1000,
      errorDismissMs: 5000,
    });
    store.notify.error("boom");
    vi.advanceTimersByTime(1000);
    expect(store.notifications()).toHaveLength(1);
    vi.advanceTimersByTime(4000);
    expect(store.notifications()).toHaveLength(0);
  });

  it("does not auto-dismiss when durations are non-positive", () => {
    vi.useFakeTimers();
    const store = createNotificationStore({
      autoDismissMs: 0,
      errorDismissMs: 0,
    });
    store.notify.info("sticky");
    store.notify.error("sticky-error");
    vi.advanceTimersByTime(60_000);
    expect(store.notifications()).toHaveLength(2);
  });
});
