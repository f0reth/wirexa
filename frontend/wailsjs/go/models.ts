export namespace adapters {
	
	export class LogEntry {
	    attrs?: Record<string, any>;
	    level: string;
	    source: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attrs = source["attrs"];
	        this.level = source["level"];
	        this.source = source["source"];
	        this.message = source["message"];
	    }
	}

}

export namespace httpdomain {
	
	export class RequestSettings {
	    proxyMode: string;
	    proxyURL: string;
	    timeoutSec: number;
	    maxResponseBodyMB: number;
	    insecureSkipVerify: boolean;
	    disableRedirects: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RequestSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proxyMode = source["proxyMode"];
	        this.proxyURL = source["proxyURL"];
	        this.timeoutSec = source["timeoutSec"];
	        this.maxResponseBodyMB = source["maxResponseBodyMB"];
	        this.insecureSkipVerify = source["insecureSkipVerify"];
	        this.disableRedirects = source["disableRedirects"];
	    }
	}
	export class KeyValuePair {
	    key: string;
	    value: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new KeyValuePair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	        this.enabled = source["enabled"];
	    }
	}
	export class RequestAuth {
	    type: string;
	    username: string;
	    password: string;
	    token: string;
	
	    static createFrom(source: any = {}) {
	        return new RequestAuth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.token = source["token"];
	    }
	}
	export class RequestBody {
	    contents: Record<string, string>;
	    type: string;
	
	    static createFrom(source: any = {}) {
	        return new RequestBody(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.contents = source["contents"];
	        this.type = source["type"];
	    }
	}
	export class HttpRequest {
	    body: RequestBody;
	    auth: RequestAuth;
	    id: string;
	    name: string;
	    method: string;
	    url: string;
	    doc: string;
	    headers: KeyValuePair[];
	    params: KeyValuePair[];
	    settings: RequestSettings;
	
	    static createFrom(source: any = {}) {
	        return new HttpRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.body = this.convertValues(source["body"], RequestBody);
	        this.auth = this.convertValues(source["auth"], RequestAuth);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.method = source["method"];
	        this.url = source["url"];
	        this.doc = source["doc"];
	        this.headers = this.convertValues(source["headers"], KeyValuePair);
	        this.params = this.convertValues(source["params"], KeyValuePair);
	        this.settings = this.convertValues(source["settings"], RequestSettings);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TreeItem {
	    request?: HttpRequest;
	    type: string;
	    id: string;
	    name: string;
	    children: TreeItem[];
	
	    static createFrom(source: any = {}) {
	        return new TreeItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.request = this.convertValues(source["request"], HttpRequest);
	        this.type = source["type"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.children = this.convertValues(source["children"], TreeItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Collection {
	    id: string;
	    name: string;
	    items: TreeItem[];
	    order: number;
	
	    static createFrom(source: any = {}) {
	        return new Collection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.items = this.convertValues(source["items"], TreeItem);
	        this.order = source["order"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class HttpResponse {
	    headers: Record<string, string>;
	    statusText: string;
	    body: string;
	    contentType: string;
	    error: string;
	    tempFilePath: string;
	    statusCode: number;
	    size: number;
	    timingMs: number;
	    bodyTruncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HttpResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.headers = source["headers"];
	        this.statusText = source["statusText"];
	        this.body = source["body"];
	        this.contentType = source["contentType"];
	        this.error = source["error"];
	        this.tempFilePath = source["tempFilePath"];
	        this.statusCode = source["statusCode"];
	        this.size = source["size"];
	        this.timingMs = source["timingMs"];
	        this.bodyTruncated = source["bodyTruncated"];
	    }
	}
	
	
	
	
	export class SidebarEntry {
	    kind: string;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new SidebarEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.id = source["id"];
	    }
	}

}

export namespace mqttdomain {
	
	export class BrokerProfile {
	    id: string;
	    name: string;
	    broker: string;
	    clientId: string;
	    username: string;
	    password: string;
	    useTls: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BrokerProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.broker = source["broker"];
	        this.clientId = source["clientId"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.useTls = source["useTls"];
	    }
	}
	export class ConnectionConfig {
	    name: string;
	    broker: string;
	    clientId: string;
	    username: string;
	    password: string;
	    useTls: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.broker = source["broker"];
	        this.clientId = source["clientId"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.useTls = source["useTls"];
	    }
	}
	export class ConnectionStatus {
	    id: string;
	    name: string;
	    broker: string;
	    connected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.broker = source["broker"];
	        this.connected = source["connected"];
	    }
	}

}

export namespace udpdomain {
	
	export class FixedLengthField {
	    name: string;
	    fieldType: string;
	    value: string;
	    length: number;
	
	    static createFrom(source: any = {}) {
	        return new FixedLengthField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.fieldType = source["fieldType"];
	        this.value = source["value"];
	        this.length = source["length"];
	    }
	}
	export class FixedLengthPayload {
	    fields: FixedLengthField[];
	
	    static createFrom(source: any = {}) {
	        return new FixedLengthPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fields = this.convertValues(source["fields"], FixedLengthField);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UdpListenSession {
	    id: string;
	    encoding: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new UdpListenSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.encoding = source["encoding"];
	        this.port = source["port"];
	    }
	}
	export class UdpSendRequest {
	    host: string;
	    encoding: string;
	    payload: string;
	    endianness: string;
	    fixedLengthPayload: FixedLengthPayload;
	    port: number;
	    messageLength: number;
	
	    static createFrom(source: any = {}) {
	        return new UdpSendRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.encoding = source["encoding"];
	        this.payload = source["payload"];
	        this.endianness = source["endianness"];
	        this.fixedLengthPayload = this.convertValues(source["fixedLengthPayload"], FixedLengthPayload);
	        this.port = source["port"];
	        this.messageLength = source["messageLength"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UdpSendResult {
	    bytesSent: number;
	
	    static createFrom(source: any = {}) {
	        return new UdpSendResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bytesSent = source["bytesSent"];
	    }
	}
	export class UdpTarget {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new UdpTarget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	    }
	}

}

