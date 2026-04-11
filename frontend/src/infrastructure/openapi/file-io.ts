import {
  OpenFilePicker,
  ReadFile,
  WriteFile,
} from "../../../wailsjs/go/adapters/OpenAPIHandler";

export async function openFilePicker(): Promise<string> {
  return OpenFilePicker();
}

export async function readFile(path: string): Promise<string> {
  return ReadFile(path);
}

export async function writeFile(path: string, content: string): Promise<void> {
  return WriteFile(path, content);
}
