import { createEffect, createSignal } from "solid-js";
import styles from "./App.module.css";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "./components/ui/resizable";
import { HttpClient } from "./presentation/components/http";
import { MqttClient } from "./presentation/components/mqtt";
import { Sidebar } from "./presentation/components/sidebar";
import {
  type Protocol,
  ProtocolSwitcher,
} from "./presentation/components/sidebar/protocol-switcher";
import { HttpProvider } from "./presentation/providers/http-provider";
import { MqttProvider } from "./presentation/providers/mqtt-provider";

function App() {
  const [protocol, setProtocol] = createSignal<Protocol>("mqtt");
  const [theme, setTheme] = createSignal<"light" | "dark">("light");

  createEffect(() => {
    if (theme() === "dark") {
      document.documentElement.classList.add("dark");
    } else {
      document.documentElement.classList.remove("dark");
    }
  });

  return (
    <div class={styles.app}>
      <HttpProvider>
        <MqttProvider>
          <ProtocolSwitcher
            protocol={protocol()}
            onProtocolChange={setProtocol}
            theme={theme()}
            onThemeToggle={() =>
              setTheme((t) => (t === "light" ? "dark" : "light"))
            }
          />
          <ResizablePanelGroup direction="horizontal">
            <ResizablePanel defaultSize={15} minSize={10}>
              <Sidebar protocol={protocol()} />
            </ResizablePanel>
            <ResizableHandle withHandle />
            <ResizablePanel defaultSize={85}>
              <main class={styles.main}>
                <div
                  class={styles.panel}
                  style={{
                    display: protocol() === "mqtt" ? "flex" : "none",
                  }}
                >
                  <MqttClient />
                </div>
                <div
                  class={styles.panel}
                  style={{
                    display: protocol() === "http" ? "flex" : "none",
                  }}
                >
                  <HttpClient />
                </div>
              </main>
            </ResizablePanel>
          </ResizablePanelGroup>
        </MqttProvider>
      </HttpProvider>
    </div>
  );
}

export default App;
