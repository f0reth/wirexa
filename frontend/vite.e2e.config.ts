import type { Plugin } from "vite";
import { defineConfig } from "vite";
import solid from "vite-plugin-solid";

function wailsMockPlugin(): Plugin {
  return {
    name: "wails-mock",
    transformIndexHtml() {
      return [
        {
          tag: "script",
          injectTo: "head-prepend",
          children: `
            const _noopVoid = () => Promise.resolve();
            const _noopNull = () => Promise.resolve(null);
            const _noopArr  = () => Promise.resolve([]);
            window.runtime = {
              EventsOnMultiple: () => () => {},
              EventsOff: () => {},
              EventsOffAll: () => {},
              EventsEmit: () => {},
              LogPrint: () => {}, LogTrace: () => {}, LogDebug: () => {},
              LogInfo: () => {}, LogWarning: () => {}, LogError: () => {}, LogFatal: () => {},
              ClipboardGetText: () => Promise.resolve(""),
              ClipboardSetText: () => Promise.resolve(true),
            };
            window.go = {
              adapters: {
                UdpHandler: {
                  DeleteTarget: _noopVoid,
                  GetListeners: _noopArr,
                  GetTargets: _noopArr,
                  SaveTarget: _noopNull,
                  Send: () => Promise.resolve({ bytesSent: 0 }),
                  Shutdown: _noopVoid,
                  StartListen: () => Promise.resolve({ id: "", port: 0, encoding: "ascii" }),
                  StopListen: _noopVoid,
                },
                HttpHandler: {
                  AddFolder: _noopNull, AddRequest: _noopNull, CancelRequest: _noopVoid,
                  CreateCollection: _noopNull, DeleteCollection: _noopVoid, DeleteItem: _noopVoid,
                  GetCollections: _noopArr, GetRootItems: _noopArr, GetSidebarLayout: _noopArr,
                  MoveCollection: _noopVoid, MoveItem: _noopVoid, MoveItemToSidebar: _noopVoid,
                  MoveSidebarEntry: _noopVoid, OpenFilePicker: () => Promise.resolve(""),
                  RenameCollection: _noopVoid, RenameItem: _noopVoid, SaveResponseBody: _noopVoid,
                  SendRequest: _noopNull, UpdateRequest: _noopVoid,
                },
                MqttHandler: {
                  Connect: _noopVoid, DeleteProfile: _noopVoid, Disconnect: _noopVoid,
                  GetConnections: _noopArr, GetProfiles: _noopArr,
                  Publish: _noopVoid, SaveProfile: _noopNull, Shutdown: _noopVoid,
                  Subscribe: _noopVoid, Unsubscribe: _noopVoid,
                },
                OpenAPIHandler: {
                  OpenFilePicker: () => Promise.resolve(""),
                  ReadFile: () => Promise.resolve(""),
                  WriteFile: _noopVoid,
                },
                LogHandler: { Log: _noopVoid },
              },
            };
          `,
        },
      ];
    },
  };
}

export default defineConfig({
  plugins: [wailsMockPlugin(), solid()],
  server: {
    port: 5173,
  },
});
