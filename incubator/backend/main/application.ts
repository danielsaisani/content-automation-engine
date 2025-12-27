import { Clock } from "@incubator/clock";
import { ApplicationConfig } from "./application-config";
import { ServiceImpl } from "./services";

export interface Application {
    listening: number;
    close(): Promise<void>;
}

export async function startApplication(applicationConfig: ApplicationConfig, clock: Clock): Promise<Application> {
    console.log("Application starting...");

    const services = new ServiceImpl();

    await services.startServices(null, null);

    return {
        listening: 3000,
        async close() {
            console.log("Application closing...");
        }
    }
}