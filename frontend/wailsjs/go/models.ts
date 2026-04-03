export namespace adapters {
	
	export class LogEntry {
	    level: string;
	    source: string;
	    message: string;
	    attrs?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.source = source["source"];
	        this.message = source["message"];
	        this.attrs = source["attrs"];
	    }
	}

}

export namespace httpdomain {
	
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
	    type: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new RequestBody(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.content = source["content"];
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
	export class HttpRequest {
	    id: string;
	    name: string;
	    method: string;
	    url: string;
	    headers: KeyValuePair[];
	    params: KeyValuePair[];
	    body: RequestBody;
	    auth: RequestAuth;
	
	    static createFrom(source: any = {}) {
	        return new HttpRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.method = source["method"];
	        this.url = source["url"];
	        this.headers = this.convertValues(source["headers"], KeyValuePair);
	        this.params = this.convertValues(source["params"], KeyValuePair);
	        this.body = this.convertValues(source["body"], RequestBody);
	        this.auth = this.convertValues(source["auth"], RequestAuth);
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
	    type: string;
	    id: string;
	    name: string;
	    children: TreeItem[];
	    request?: HttpRequest;
	
	    static createFrom(source: any = {}) {
	        return new TreeItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.id = source["id"];
	        this.name = source["name"];
	        this.children = this.convertValues(source["children"], TreeItem);
	        this.request = this.convertValues(source["request"], HttpRequest);
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
	
	    static createFrom(source: any = {}) {
	        return new Collection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.items = this.convertValues(source["items"], TreeItem);
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
	    statusCode: number;
	    statusText: string;
	    headers: Record<string, string>;
	    body: string;
	    contentType: string;
	    size: number;
	    timingMs: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new HttpResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.statusCode = source["statusCode"];
	        this.statusText = source["statusText"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.contentType = source["contentType"];
	        this.size = source["size"];
	        this.timingMs = source["timingMs"];
	        this.error = source["error"];
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
	    length: number;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new FixedLengthField(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.length = source["length"];
	        this.value = source["value"];
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
	    port: number;
	    encoding: string;
	
	    static createFrom(source: any = {}) {
	        return new UdpListenSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.port = source["port"];
	        this.encoding = source["encoding"];
	    }
	}
	export class UdpSendRequest {
	    host: string;
	    port: number;
	    encoding: string;
	    payload: string;
	    messageLength: number;
	    fixedLengthPayload: FixedLengthPayload;
	
	    static createFrom(source: any = {}) {
	        return new UdpSendRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.host = source["host"];
	        this.port = source["port"];
	        this.encoding = source["encoding"];
	        this.payload = source["payload"];
	        this.messageLength = source["messageLength"];
	        this.fixedLengthPayload = this.convertValues(source["fixedLengthPayload"], FixedLengthPayload);
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

