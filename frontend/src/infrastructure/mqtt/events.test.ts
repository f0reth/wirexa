import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("../../../wailsjs/runtime/runtime", () => ({
  EventsOn: vi.fn(),
}));

import * as Runtime from "../../../wailsjs/runtime/runtime";
import type { MqttEventName } from "./events";
import { onMqttEvent } from "./events";

const MQTT_EVENTS: MqttEventName[] = [
  "mqtt:connected",
  "mqtt:disconnected",
  "mqtt:connection-lost",
  "mqtt:connection-failed",
  "mqtt:message",
];

beforeEach(() => {
  vi.clearAllMocks();
});

describe("onMqttEvent", () => {
  it("calls EventsOn with the given event name and handler", () => {
    const mockCleanup = vi.fn();
    vi.mocked(Runtime.EventsOn).mockReturnValue(mockCleanup);

    const handler = vi.fn();
    onMqttEvent("mqtt:connected", handler);

    expect(Runtime.EventsOn).toHaveBeenCalledWith("mqtt:connected", handler);
  });

  it("returns the cleanup function provided by EventsOn", () => {
    const mockCleanup = vi.fn();
    vi.mocked(Runtime.EventsOn).mockReturnValue(mockCleanup);

    const cleanup = onMqttEvent("mqtt:connected", vi.fn());

    expect(cleanup).toBe(mockCleanup);
  });

  it.each(MQTT_EVENTS)("registers a handler for %s", (event) => {
    const mockCleanup = vi.fn();
    vi.mocked(Runtime.EventsOn).mockReturnValue(mockCleanup);

    const handler = vi.fn();
    onMqttEvent(event, handler);

    expect(Runtime.EventsOn).toHaveBeenCalledWith(event, handler);
  });

  it("forwards data to the handler when the event fires", () => {
    const handler = vi.fn();
    let capturedCallback: ((data: unknown) => void) | undefined;

    vi.mocked(Runtime.EventsOn).mockImplementation((_name, cb) => {
      capturedCallback = cb as (data: unknown) => void;
      return vi.fn();
    });

    onMqttEvent("mqtt:message", handler);

    const payload = { topic: "home/temp", payload: "22.5" };
    capturedCallback?.(payload);

    expect(handler).toHaveBeenCalledWith(payload);
  });

  it("calls EventsOn exactly once per registration", () => {
    vi.mocked(Runtime.EventsOn).mockReturnValue(vi.fn());
    onMqttEvent("mqtt:connected", vi.fn());
    expect(Runtime.EventsOn).toHaveBeenCalledOnce();
  });

  it("supports multiple independent registrations for the same event", () => {
    vi.mocked(Runtime.EventsOn).mockReturnValue(vi.fn());
    onMqttEvent("mqtt:message", vi.fn());
    onMqttEvent("mqtt:message", vi.fn());
    expect(Runtime.EventsOn).toHaveBeenCalledTimes(2);
  });
});
