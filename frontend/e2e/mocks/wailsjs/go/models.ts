export namespace adapters {
  export interface LogEntry {
    level: string;
    source: string;
    message: string;
    attrs?: Record<string, unknown>;
  }
}

export namespace httpdomain {
  export interface KeyValuePair {
    key: string;
    value: string;
    enabled: boolean;
  }

  export interface RequestBody {
    type: string;
    content: string;
  }

  export interface RequestAuth {
    type: string;
    username: string;
    password: string;
    token: string;
  }

  export interface HttpRequest {
    id: string;
    name: string;
    method: string;
    url: string;
    headers: KeyValuePair[];
    params: KeyValuePair[];
    body: RequestBody;
    auth: RequestAuth;
  }

  export interface TreeItem {
    type: string;
    id: string;
    name: string;
    children: TreeItem[];
    request?: HttpRequest;
  }

  export interface Collection {
    id: string;
    name: string;
    items: TreeItem[];
  }

  export interface HttpResponse {
    statusCode: number;
    statusText: string;
    headers: Record<string, string>;
    body: string;
    contentType: string;
    size: number;
    timingMs: number;
    error: string;
  }
}

export namespace mqttdomain {
  export interface BrokerProfile {
    id: string;
    name: string;
    broker: string;
    clientId: string;
    username: string;
    password: string;
    useTls: boolean;
  }

  export interface ConnectionConfig {
    name: string;
    broker: string;
    clientId: string;
    username: string;
    password: string;
    useTls: boolean;
  }

  export interface ConnectionStatus {
    id: string;
    name: string;
    broker: string;
    connected: boolean;
  }
}

export namespace udpdomain {
  export interface FixedLengthField {
    name: string;
    length: number;
    value: string;
  }

  export interface FixedLengthPayload {
    fields: FixedLengthField[];
  }

  export interface UdpListenSession {
    id: string;
    port: number;
    encoding: string;
  }

  export interface UdpSendRequest {
    host: string;
    port: number;
    encoding: string;
    payload: string;
    messageLength: number;
    fixedLengthPayload: FixedLengthPayload;
  }

  export interface UdpSendResult {
    bytesSent: number;
  }

  export interface UdpTarget {
    id: string;
    name: string;
    host: string;
    port: number;
  }
}
