export interface Services {
    startServices(serviceFactories: any, dependencies: any): Promise<void>;
    stopServices(): Promise<void>;
    getServiceInstance(name: string): any;
}

export class ServiceImpl implements Services {
    public async startServices(
        serviceFactories: any,
        dependencies: any
    ): Promise<void> {
        console.log("Services starting...");
    }

    public async stopServices(): Promise<void> {
        console.log("Services stopping...");
    }

    public getServiceInstance(name: string): any {
        console.log(`Getting service instance for: ${name}`);
        return {};
    }
}