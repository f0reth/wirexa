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

  it("uses 4s / 8s / 5 as defaults", () => {
    vi.useFakeTimers();
    const store = createNotificationStore();
    store.notify.info("info");
    store.notify.error("error");

    vi.advanceTimersByTime(3999);
    expect(store.notifications()).toHaveLength(2);
    vi.advanceTimersByTime(1);
    expect(store.notifications().map((n) => n.title)).toEqual(["error"]);
    vi.advanceTimersByTime(3999);
    expect(store.notifications()).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(store.notifications()).toHaveLength(0);

    for (let i = 1; i <= 6; i++) store.notify.info(String(i));
    expect(store.notifications().map((n) => n.title)).toEqual([
      "2",
      "3",
      "4",
      "5",
      "6",
    ]);
  });

  it("records every field including createdAt", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-09-27T12:00:00Z"));
    const store = createNotificationStore();

    const id = store.notify.warning("title", "desc");

    expect(store.notifications()).toEqual([
      {
        id,
        level: "warning",
        title: "title",
        description: "desc",
        createdAt: new Date("2026-09-27T12:00:00Z").getTime(),
      },
    ]);
  });

  // key の解放漏れがあると、その key の通知（MQTT の connection-lost など）が以後出なくなる。
  it("releases the key of a notification dropped by max", () => {
    vi.useFakeTimers();
    const store = createNotificationStore({ max: 1 });
    const first = store.notify.error("lost", undefined, { key: "conn-1" });
    store.notify.info("other");
    expect(store.notifications().map((n) => n.title)).toEqual(["other"]);

    const second = store.notify.error("lost", undefined, { key: "conn-1" });
    expect(second).not.toBe(first);
    expect(store.notifications().map((n) => n.title)).toEqual(["lost"]);
    // 押し出された通知のタイマーも残さない（残るのは表示中の 1 件分だけ）。
    expect(vi.getTimerCount()).toBe(1);
  });

  it("releases the key after auto-dismiss", () => {
    vi.useFakeTimers();
    const store = createNotificationStore({ errorDismissMs: 1000 });
    const first = store.notify.error("lost", undefined, { key: "conn-1" });
    vi.advanceTimersByTime(1000);
    expect(store.notifications()).toHaveLength(0);

    const second = store.notify.error("lost", undefined, { key: "conn-1" });
    expect(second).not.toBe(first);
    expect(store.notifications()).toHaveLength(1);
  });

  it("releases keys and timers on clear", () => {
    vi.useFakeTimers();
    const store = createNotificationStore();
    const first = store.notify.error("lost", undefined, { key: "conn-1" });
    store.notify.info("other");
    expect(vi.getTimerCount()).toBe(2);

    store.clear();
    expect(vi.getTimerCount()).toBe(0);

    const second = store.notify.error("lost", undefined, { key: "conn-1" });
    expect(second).not.toBe(first);
    expect(store.notifications()).toHaveLength(1);
  });
});
