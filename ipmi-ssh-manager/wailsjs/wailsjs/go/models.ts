export namespace domain {
	
	export class ExecHistory {
	    id: number;
	    machine_id: number;
	    ipmi_ip: string;
	    command: string;
	    stdout: string;
	    stderr: string;
	    exit_code: number;
	    error?: string;
	    // Go type: time
	    started_at: any;
	    // Go type: time
	    finished_at: any;
	    duration_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new ExecHistory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.machine_id = source["machine_id"];
	        this.ipmi_ip = source["ipmi_ip"];
	        this.command = source["command"];
	        this.stdout = source["stdout"];
	        this.stderr = source["stderr"];
	        this.exit_code = source["exit_code"];
	        this.error = source["error"];
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.finished_at = this.convertValues(source["finished_at"], null);
	        this.duration_ms = source["duration_ms"];
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
	export class ExecResult {
	    MachineID: number;
	    IPMIIP: string;
	    SSHIP: string;
	    SSHUser: string;
	    Stdout: string;
	    Stderr: string;
	    ExitCode: number;
	    Err: any;
	
	    static createFrom(source: any = {}) {
	        return new ExecResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.MachineID = source["MachineID"];
	        this.IPMIIP = source["IPMIIP"];
	        this.SSHIP = source["SSHIP"];
	        this.SSHUser = source["SSHUser"];
	        this.Stdout = source["Stdout"];
	        this.Stderr = source["Stderr"];
	        this.ExitCode = source["ExitCode"];
	        this.Err = source["Err"];
	    }
	}
	export class Machine {
	    id: number;
	    ipmi_ip: string;
	    ssh_ip: string;
	    ssh_user: string;
	    remark?: string;
	    // Go type: time
	    created_at?: any;
	
	    static createFrom(source: any = {}) {
	        return new Machine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ipmi_ip = source["ipmi_ip"];
	        this.ssh_ip = source["ssh_ip"];
	        this.ssh_user = source["ssh_user"];
	        this.remark = source["remark"];
	        this.created_at = this.convertValues(source["created_at"], null);
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

}

