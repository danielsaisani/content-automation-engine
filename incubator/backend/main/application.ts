import { ApplicationConfig } from "./application-config";

export interface Application {
    listening: number;
    close(): Promise<void>;
}

export async function startApplication(applicationConfig: ApplicationConfig): Promise<Application> {
    console.log("Application starting...");

    return {
        listening: 3000,
        async close() {
            console.log("Application closing...");
        }
    }
}