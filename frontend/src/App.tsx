import Resizable from "@corvu/resizable";
import { createEffect, createSignal, Show } from "solid-js";
import styles from "./App.module.css";
import { createThemeStore } from "./application/ui/theme";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "./components/ui/resizable";
import { createThemeStorage } from "./infrastructure/storage/local-storage";
import { HttpClient } from "./presentation/components/http";
import { MqttClient } from "./presentation/components/mqtt";
import { OpenApiClient } from "./presentation/components/openapi";
import { Sidebar } from "./presentation/components/sidebar";
import {
  type Protocol,
  ProtocolSwitcher,
} from "./presentation/components/sidebar/protocol-switcher";
import { UdpClient } from "./presentation/components/udp";
import { HttpProvider } from "./presentation/providers/http-provider";
import { MqttProvider } from "./presentation/providers/mqtt-provider";
import { OpenApiProvider } from "./presentation/providers/openapi-provider";
import { UdpProvider } from "./presentation/providers/udp-provider";

function SidebarCollapseController(props: { sidebarOpen: () => boolean }) {
  const panelCtx = Resizable.usePanelContext();
  createEffect(() => {
    if (props.sidebarOpen()) {
      if (panelCtx.collapsed()) panelCtx.expand();
    } else {
      if (!panelCtx.collapsed()) panelCtx.collapse();
    }
  });
  return null;
}

function App() {
  const [protocol, setProtocol] = createSignal<Protocol>("mqtt");
  const [sidebarOpen, setSidebarOpen] = createSignal(true);
  const { theme, toggleTheme } = createThemeStore(createThemeStorage());

  const handleProtocolChange = (p: Protocol) => {
    if (p === protocol()) {
      setSidebarOpen((prev) => !prev);
    } else {
      setProtocol(p);
      setSidebarOpen(true);
    }
  };

  // Track which protocols have been visited; mount on first visit, keep alive thereafter
  const [visited, setVisited] = createSignal<Set<Protocol>>(
    new Set<Protocol>(["mqtt"]),
  );

  createEffect(() => {
    setVisited((prev) => new Set([...prev, protocol()]));
  });

  createEffect(() => {
    if (theme() === "dark") {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  });

  return (
    <div class={styles.app}>
      <UdpProvider>
        <HttpProvider>
          <MqttProvider>
            <OpenApiProvider>
              <ProtocolSwitcher
                protocol={protocol()}
                sidebarOpen={sidebarOpen()}
                onProtocolChange={handleProtocolChange}
                theme={theme()}
                onThemeToggle={toggleTheme}
              />
              <ResizablePanelGroup direction="horizontal">
                <ResizablePanel defaultSize={15} minSize={10} collapsible>
                  <SidebarCollapseController sidebarOpen={sidebarOpen} />
                  <Sidebar protocol={protocol()} />
                </ResizablePanel>
                <ResizableHandle withHandle />
                <ResizablePanel defaultSize={85}>
                  <main class={styles.main}>
                    <Show when={visited().has("mqtt")}>
                      <div
                        data-testid="mqtt-panel"
                        class={styles.panel}
                        style={{
                          display: protocol() === "mqtt" ? "flex" : "none",
                        }}
                      >
                        <MqttClient />
                      </div>
                    </Show>
                    <Show when={visited().has("http")}>
                      <div
                        data-testid="http-panel"
                        class={styles.panel}
                        style={{
                          display: protocol() === "http" ? "flex" : "none",
                        }}
                      >
                        <HttpClient />
                      </div>
                    </Show>
                    <Show when={visited().has("udp")}>
                      <div
                        data-testid="udp-panel"
                        class={styles.panel}
                        style={{
                          display: protocol() === "udp" ? "flex" : "none",
                        }}
                      >
                        <UdpClient />
                      </div>
                    </Show>
                    <Show when={visited().has("openapi")}>
                      <div
                        data-testid="openapi-panel"
                        class={styles.panel}
                        style={{
                          display: protocol() === "openapi" ? "flex" : "none",
                        }}
                      >
                        <OpenApiClient />
                      </div>
                    </Show>
                  </main>
                </ResizablePanel>
              </ResizablePanelGroup>
            </OpenApiProvider>
          </MqttProvider>
        </HttpProvider>
      </UdpProvider>
    </div>
  );
}

export default App;
